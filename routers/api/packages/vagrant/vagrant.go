// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package vagrant

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	vagrant_module "code.gitea.io/gitea/modules/packages/vagrant"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, struct {
			Errors []string `json:"errors"`
		}{
			Errors: []string{
				message,
			},
		})
	})
}

func CheckAuthenticate(ctx *context.Context) {
	if ctx.Doer == nil {
		apiError(ctx, http.StatusUnauthorized, "Invalid access token")
		return
	}

	ctx.Status(http.StatusOK)
}

func CheckBoxAvailable(ctx *context.Context) {
	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeVagrant, ctx.Params("name"))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	ctx.JSON(http.StatusOK, nil) // needs to be Content-Type: application/json
}

type packageMetadata struct {
	Name             string             `json:"name"`
	Description      string             `json:"description,omitempty"`
	ShortDescription string             `json:"short_description,omitempty"`
	Versions         []*versionMetadata `json:"versions"`
}

type versionMetadata struct {
	Version             string          `json:"version"`
	Status              string          `json:"status"`
	DescriptionHTML     string          `json:"description_html,omitempty"`
	DescriptionMarkdown string          `json:"description_markdown,omitempty"`
	Providers           []*providerData `json:"providers"`
}

type providerData struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Checksum     string `json:"checksum"`
	ChecksumType string `json:"checksum_type"`
}

func packageDescriptorToMetadata(baseURL string, pd *packages_model.PackageDescriptor) *versionMetadata {
	versionURL := baseURL + "/" + url.PathEscape(pd.Version.Version)

	providers := make([]*providerData, 0, len(pd.Files))

	for _, f := range pd.Files {
		providers = append(providers, &providerData{
			Name:         f.Properties.GetByName(vagrant_module.PropertyProvider),
			URL:          versionURL + "/" + url.PathEscape(f.File.Name),
			Checksum:     f.Blob.HashSHA512,
			ChecksumType: "sha512",
		})
	}

	return &versionMetadata{
		Status:    "active",
		Version:   pd.Version.Version,
		Providers: providers,
	}
}

func EnumeratePackageVersions(ctx *context.Context) {
	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeVagrant, ctx.Params("name"))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	sort.Slice(pds, func(i, j int) bool {
		return pds[i].SemVer.LessThan(pds[j].SemVer)
	})

	baseURL := fmt.Sprintf("%sapi/packages/%s/vagrant/%s", setting.AppURL, url.PathEscape(ctx.Package.Owner.Name), url.PathEscape(pds[0].Package.Name))

	versions := make([]*versionMetadata, 0, len(pds))
	for _, pd := range pds {
		versions = append(versions, packageDescriptorToMetadata(baseURL, pd))
	}

	ctx.JSON(http.StatusOK, &packageMetadata{
		Name:        pds[0].Package.Name,
		Description: pds[len(pds)-1].Metadata.(*vagrant_module.Metadata).Description,
		Versions:    versions,
	})
}

func UploadPackageFile(ctx *context.Context) {
	boxName := ctx.Params("name")
	boxVersion := ctx.Params("version")
	_, err := version.NewSemver(boxVersion)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	boxProvider := ctx.Params("provider")
	if !strings.HasSuffix(boxProvider, ".box") {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	upload, needsClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if needsClose {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	metadata, err := vagrant_module.ParseMetadataFromBox(buf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeVagrant,
				Name:        boxName,
				Version:     boxVersion,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(boxProvider),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				vagrant_module.PropertyProvider: strings.TrimSuffix(boxProvider, ".box"),
			},
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

func DownloadPackageFile(ctx *context.Context) {
	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeVagrant,
			Name:        ctx.Params("name"),
			Version:     ctx.Params("version"),
		},
		&packages_service.PackageFileInfo{
			Filename: ctx.Params("provider"),
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}
