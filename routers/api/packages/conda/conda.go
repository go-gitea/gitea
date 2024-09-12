// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conda

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	conda_model "code.gitea.io/gitea/models/packages/conda"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	conda_module "code.gitea.io/gitea/modules/packages/conda"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/dsnet/compress/bzip2"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		}{
			Reason:  http.StatusText(status),
			Message: message,
		})
	})
}

func EnumeratePackages(ctx *context.Context) {
	type Info struct {
		Subdir string `json:"subdir"`
	}

	type PackageInfo struct {
		Name          string   `json:"name"`
		Version       string   `json:"version"`
		NoArch        string   `json:"noarch"`
		Subdir        string   `json:"subdir"`
		Timestamp     int64    `json:"timestamp"`
		Build         string   `json:"build"`
		BuildNumber   int64    `json:"build_number"`
		Dependencies  []string `json:"depends"`
		License       string   `json:"license"`
		LicenseFamily string   `json:"license_family"`
		HashMD5       string   `json:"md5"`
		HashSHA256    string   `json:"sha256"`
		Size          int64    `json:"size"`
	}

	type RepoData struct {
		Info          Info                    `json:"info"`
		Packages      map[string]*PackageInfo `json:"packages"`
		PackagesConda map[string]*PackageInfo `json:"packages.conda"`
		Removed       map[string]*PackageInfo `json:"removed"`
	}

	repoData := &RepoData{
		Info: Info{
			Subdir: ctx.PathParam("architecture"),
		},
		Packages:      make(map[string]*PackageInfo),
		PackagesConda: make(map[string]*PackageInfo),
		Removed:       make(map[string]*PackageInfo),
	}

	pfs, err := conda_model.SearchFiles(ctx, &conda_model.FileSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Channel: ctx.PathParam("channel"),
		Subdir:  repoData.Info.Subdir,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pfs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pds := make(map[int64]*packages_model.PackageDescriptor)

	for _, pf := range pfs {
		pd, exists := pds[pf.VersionID]
		if !exists {
			pv, err := packages_model.GetVersionByID(ctx, pf.VersionID)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}

			pd, err = packages_model.GetPackageDescriptor(ctx, pv)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}

			pds[pf.VersionID] = pd
		}

		var pfd *packages_model.PackageFileDescriptor
		for _, d := range pd.Files {
			if d.File.ID == pf.ID {
				pfd = d
				break
			}
		}

		var fileMetadata *conda_module.FileMetadata
		if err := json.Unmarshal([]byte(pfd.Properties.GetByName(conda_module.PropertyMetadata)), &fileMetadata); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		versionMetadata := pd.Metadata.(*conda_module.VersionMetadata)

		pi := &PackageInfo{
			Name:          pd.PackageProperties.GetByName(conda_module.PropertyName),
			Version:       pd.Version.Version,
			NoArch:        fileMetadata.NoArch,
			Subdir:        repoData.Info.Subdir,
			Timestamp:     fileMetadata.Timestamp,
			Build:         fileMetadata.Build,
			BuildNumber:   fileMetadata.BuildNumber,
			Dependencies:  fileMetadata.Dependencies,
			License:       versionMetadata.License,
			LicenseFamily: versionMetadata.LicenseFamily,
			HashMD5:       pfd.Blob.HashMD5,
			HashSHA256:    pfd.Blob.HashSHA256,
			Size:          pfd.Blob.Size,
		}

		if fileMetadata.IsCondaPackage {
			repoData.PackagesConda[pfd.File.Name] = pi
		} else {
			repoData.Packages[pfd.File.Name] = pi
		}
	}

	resp := ctx.Resp

	var w io.Writer = resp

	if strings.HasSuffix(ctx.PathParam("filename"), ".json") {
		resp.Header().Set("Content-Type", "application/json")
	} else {
		resp.Header().Set("Content-Type", "application/x-bzip2")

		zw, err := bzip2.NewWriter(w, nil)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		defer zw.Close()

		w = zw
	}

	resp.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(repoData); err != nil {
		log.Error("JSON encode: %v", err)
	}
}

func UploadPackageFile(ctx *context.Context) {
	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if needToClose {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	var pck *conda_module.Package
	if strings.HasSuffix(strings.ToLower(ctx.PathParam("filename")), ".tar.bz2") {
		pck, err = conda_module.ParsePackageBZ2(buf)
	} else {
		pck, err = conda_module.ParsePackageConda(buf, buf.Size())
	}
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

	fullName := pck.Name

	channel := ctx.PathParam("channel")
	if channel != "" {
		fullName = channel + "/" + pck.Name
	}

	extension := ".tar.bz2"
	if pck.FileMetadata.IsCondaPackage {
		extension = ".conda"
	}

	fileMetadataRaw, err := json.Marshal(pck.FileMetadata)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeConda,
				Name:        fullName,
				Version:     pck.Version,
			},
			SemverCompatible: false,
			Creator:          ctx.Doer,
			Metadata:         pck.VersionMetadata,
			PackageProperties: map[string]string{
				conda_module.PropertyName:    pck.Name,
				conda_module.PropertyChannel: channel,
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s-%s-%s%s", pck.Name, pck.Version, pck.FileMetadata.Build, extension),
				CompositeKey: pck.Subdir,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				conda_module.PropertySubdir:   pck.Subdir,
				conda_module.PropertyMetadata: string(fileMetadataRaw),
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
	pfs, err := conda_model.SearchFiles(ctx, &conda_model.FileSearchOptions{
		OwnerID:  ctx.Package.Owner.ID,
		Channel:  ctx.PathParam("channel"),
		Subdir:   ctx.PathParam("architecture"),
		Filename: ctx.PathParam("filename"),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pf := pfs[0]

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}
