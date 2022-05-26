// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package helm

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	helm_module "code.gitea.io/gitea/modules/packages/helm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"

	"gopkg.in/yaml.v3"
)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		type Error struct {
			Error string `json:"error"`
		}
		ctx.JSON(status, Error{
			Error: message,
		})
	})
}

// Index generates the Helm charts index
func Index(ctx *context.Context) {
	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeHelm,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	baseURL := setting.AppURL + "api/packages/" + url.PathEscape(ctx.Package.Owner.Name) + "/helm"

	type ChartVersion struct {
		helm_module.Metadata `yaml:",inline"`
		URLs                 []string  `yaml:"urls"`
		Created              time.Time `yaml:"created,omitempty"`
		Removed              bool      `yaml:"removed,omitempty"`
		Digest               string    `yaml:"digest,omitempty"`
	}

	type ServerInfo struct {
		ContextPath string `yaml:"contextPath,omitempty"`
	}

	type Index struct {
		APIVersion string                     `yaml:"apiVersion"`
		Entries    map[string][]*ChartVersion `yaml:"entries"`
		Generated  time.Time                  `yaml:"generated,omitempty"`
		ServerInfo *ServerInfo                `yaml:"serverInfo,omitempty"`
	}

	entries := make(map[string][]*ChartVersion)
	for _, pv := range pvs {
		metadata := &helm_module.Metadata{}
		if err := json.Unmarshal([]byte(pv.MetadataJSON), &metadata); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		entries[metadata.Name] = append(entries[metadata.Name], &ChartVersion{
			Metadata: *metadata,
			Created:  pv.CreatedUnix.AsTime(),
			URLs:     []string{fmt.Sprintf("%s/%s", baseURL, url.PathEscape(createFilename(metadata)))},
		})
	}

	ctx.Resp.WriteHeader(http.StatusOK)
	if err := yaml.NewEncoder(ctx.Resp).Encode(&Index{
		APIVersion: "v1",
		Entries:    entries,
		Generated:  time.Now(),
		ServerInfo: &ServerInfo{
			ContextPath: setting.AppSubURL + "/api/packages/" + url.PathEscape(ctx.Package.Owner.Name) + "/helm",
		},
	}); err != nil {
		log.Error("YAML encode failed: %v", err)
	}
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.Context) {
	filename := ctx.Params("filename")

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeHelm,
		Name: packages_model.SearchValue{
			ExactMatch: true,
			Value:      ctx.Params("package"),
		},
		HasFileWithName: filename,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	s, pf, err := packages_service.GetFileStreamByPackageVersion(
		ctx,
		pvs[0],
		&packages_service.PackageFileInfo{
			Filename: filename,
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackage creates a new package
func UploadPackage(ctx *context.Context) {
	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if needToClose {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	metadata, err := helm_module.ParseChartArchive(buf)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
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
				PackageType: packages_model.TypeHelm,
				Name:        metadata.Name,
				Version:     metadata.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: createFilename(metadata),
			},
			Data:              buf,
			IsLead:            true,
			OverwriteExisting: true,
		},
	)
	if err != nil {
		if err == packages_model.ErrDuplicatePackageVersion {
			apiError(ctx, http.StatusConflict, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

func createFilename(metadata *helm_module.Metadata) string {
	return strings.ToLower(fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version))
}
