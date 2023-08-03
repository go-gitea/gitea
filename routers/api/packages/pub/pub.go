// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pub

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	pub_module "code.gitea.io/gitea/modules/packages/pub"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
)

func jsonResponse(ctx *context.Context, status int, obj any) {
	resp := ctx.Resp
	resp.Header().Set("Content-Type", "application/vnd.pub.v2+json")
	resp.WriteHeader(status)
	if err := json.NewEncoder(resp).Encode(obj); err != nil {
		log.Error("JSON encode: %v", err)
	}
}

func apiError(ctx *context.Context, status int, obj any) {
	type Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type ErrorWrapper struct {
		Error Error `json:"error"`
	}

	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		jsonResponse(ctx, status, ErrorWrapper{
			Error: Error{
				Code:    http.StatusText(status),
				Message: message,
			},
		})
	})
}

type packageVersions struct {
	Name     string             `json:"name"`
	Latest   *versionMetadata   `json:"latest"`
	Versions []*versionMetadata `json:"versions"`
}

type versionMetadata struct {
	Version    string    `json:"version"`
	ArchiveURL string    `json:"archive_url"`
	Published  time.Time `json:"published"`
	Pubspec    any       `json:"pubspec,omitempty"`
}

func packageDescriptorToMetadata(baseURL string, pd *packages_model.PackageDescriptor) *versionMetadata {
	return &versionMetadata{
		Version:    pd.Version.Version,
		ArchiveURL: fmt.Sprintf("%s/files/%s.tar.gz", baseURL, url.PathEscape(pd.Version.Version)),
		Published:  pd.Version.CreatedUnix.AsLocalTime(),
		Pubspec:    pd.Metadata.(*pub_module.Metadata).Pubspec,
	}
}

func baseURL(ctx *context.Context) string {
	return setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/pub/api/packages"
}

// https://github.com/dart-lang/pub/blob/master/doc/repository-spec-v2.md#list-all-versions-of-a-package
func EnumeratePackageVersions(ctx *context.Context) {
	packageName := ctx.Params("id")

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypePub, packageName)
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

	baseURL := fmt.Sprintf("%s/%s", baseURL(ctx), url.PathEscape(pds[0].Package.Name))

	versions := make([]*versionMetadata, 0, len(pds))
	for _, pd := range pds {
		versions = append(versions, packageDescriptorToMetadata(baseURL, pd))
	}

	jsonResponse(ctx, http.StatusOK, &packageVersions{
		Name:     pds[0].Package.Name,
		Latest:   packageDescriptorToMetadata(baseURL, pds[0]),
		Versions: versions,
	})
}

// https://github.com/dart-lang/pub/blob/master/doc/repository-spec-v2.md#deprecated-inspect-a-specific-version-of-a-package
func PackageVersionMetadata(ctx *context.Context) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypePub, packageName, packageVersion)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	jsonResponse(ctx, http.StatusOK, packageDescriptorToMetadata(
		fmt.Sprintf("%s/%s", baseURL(ctx), url.PathEscape(pd.Package.Name)),
		pd,
	))
}

// https://github.com/dart-lang/pub/blob/master/doc/repository-spec-v2.md#publishing-packages
func RequestUpload(ctx *context.Context) {
	type UploadRequest struct {
		URL    string            `json:"url"`
		Fields map[string]string `json:"fields"`
	}

	jsonResponse(ctx, http.StatusOK, UploadRequest{
		URL:    baseURL(ctx) + "/versions/new/upload",
		Fields: make(map[string]string),
	})
}

// https://github.com/dart-lang/pub/blob/master/doc/repository-spec-v2.md#publishing-packages
func UploadPackageFile(ctx *context.Context) {
	file, _, err := ctx.Req.FormFile("file")
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	buf, err := packages_module.CreateHashedBufferFromReader(file)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pck, err := pub_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, _, err = packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypePub,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         pck.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(pck.Version + ".tar.gz"),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion:
			apiError(ctx, http.StatusBadRequest, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Resp.Header().Set("Location", fmt.Sprintf("%s/versions/new/finalize/%s/%s", baseURL(ctx), url.PathEscape(pck.Name), url.PathEscape(pck.Version)))
	ctx.Status(http.StatusNoContent)
}

// https://github.com/dart-lang/pub/blob/master/doc/repository-spec-v2.md#publishing-packages
func FinalizePackage(ctx *context.Context) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")

	_, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypePub, packageName, packageVersion)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type Success struct {
		Message string `json:"message"`
	}
	type SuccessWrapper struct {
		Success Success `json:"success"`
	}

	jsonResponse(ctx, http.StatusOK, SuccessWrapper{Success{}})
}

// https://github.com/dart-lang/pub/blob/master/doc/repository-spec-v2.md#deprecated-download-a-specific-version-of-a-package
func DownloadPackageFile(ctx *context.Context) {
	packageName := ctx.Params("id")
	packageVersion := strings.TrimSuffix(ctx.Params("version"), ".tar.gz")

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypePub, packageName, packageVersion)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pf := pd.Files[0].File

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}
