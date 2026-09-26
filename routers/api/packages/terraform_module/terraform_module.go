// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	packages_model "gitea.dev/models/packages"
	packages_module "gitea.dev/modules/packages"
	tfmod "gitea.dev/modules/packages/terraform_module"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/packages/helper"
	"gitea.dev/services/context"
	packages_service "gitea.dev/services/packages"
)

const (
	archiveFilename    = "module.tar.gz"
	archiveURLLifetime = 10 * time.Minute
)

func apiError(ctx *context.Context, status int, obj any) {
	message := helper.ProcessErrorForUser(ctx, status, obj)
	ctx.PlainText(status, message)
}

func packageName(ctx *context.Context) (string, error) {
	name, provider := ctx.PathParam("name"), ctx.PathParam("provider")
	if err := tfmod.ValidateNameAndProvider(name, provider); err != nil {
		return "", err
	}
	return name + "/" + provider, nil
}

func packageInfo(ctx *context.Context) (*packages_service.PackageInfo, error) {
	name, err := packageName(ctx)
	if err != nil {
		return nil, err
	}
	version, err := tfmod.NormalizeVersion(ctx.PathParam("version"))
	if err != nil {
		return nil, err
	}
	return &packages_service.PackageInfo{
		Owner:       ctx.Package.Owner,
		PackageType: packages_model.TypeTerraformModule,
		Name:        name,
		Version:     version,
	}, nil
}

func archiveSignature(pi *packages_service.PackageInfo, expires string) string {
	mac := hmac.New(sha256.New, setting.GetGeneralTokenSigningSecret())
	_, _ = fmt.Fprintf(mac, "terraform-module:%d:%s:%s:%s", pi.Owner.ID, pi.Name, pi.Version, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func HasValidArchiveSignature(ctx *context.Context) bool {
	pi, err := packageInfo(ctx)
	expires := ctx.FormString("expires")
	expiresUnix, _ := strconv.ParseInt(expires, 10, 64)
	return err == nil && time.Now().Unix() < expiresUnix && hmac.Equal([]byte(ctx.FormString("sig")), []byte(archiveSignature(pi, expires)))
}

func ServiceDiscovery(ctx *context.Context) {
	ctx.JSON(http.StatusOK, map[string]string{"modules.v1": setting.AppSubURL + "/api/packages/-/terraform/modules/"})
}

// https://developer.hashicorp.com/terraform/internals/module-registry-protocol#list-available-versions-for-a-specific-module
func ListVersions(ctx *context.Context) {
	name, err := packageName(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformModule, name)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, packages_model.ErrPackageNotExist)
		return
	}
	versions := make([]map[string]string, 0, len(pvs))
	for _, pv := range pvs {
		versions = append(versions, map[string]string{"version": pv.Version})
	}
	ctx.JSON(http.StatusOK, map[string]any{"modules": []any{map[string]any{"versions": versions}}})
}

// https://developer.hashicorp.com/terraform/internals/module-registry-protocol#download-source-code-for-a-specific-module-version
func DownloadRedirect(ctx *context.Context) {
	pi, err := packageInfo(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	if _, err := packages_model.GetVersionByNameAndVersion(ctx, pi.Owner.ID, pi.PackageType, pi.Name, pi.Version); err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	// signed because Terraform sends no credentials when fetching it, "archive" tells go-getter the format of the extensionless URL
	expires := strconv.FormatInt(time.Now().Add(archiveURLLifetime).Unix(), 10)
	ctx.Resp.Header().Set("X-Terraform-Get", "./archive?archive=tar.gz&expires="+expires+"&sig="+archiveSignature(pi, expires))
	ctx.Status(http.StatusNoContent)
}

func DownloadArchive(ctx *context.Context) {
	pi, err := packageInfo(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	s, u, pf, err := packages_service.OpenFileForDownloadByPackageNameAndVersion(ctx, pi, &packages_service.PackageFileInfo{Filename: archiveFilename}, ctx.Req.Method)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) || errors.Is(err, packages_model.ErrPackageFileNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	helper.ServePackageFile(ctx, s, u, pf)
}

func UploadModule(ctx *context.Context) {
	pi, err := packageInfo(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

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

	metadata, err := tfmod.ParseModuleArchive(buf)
	if err != nil {
		apiError(ctx, util.Iif(errors.Is(err, tfmod.ErrArchiveTooLarge), http.StatusRequestEntityTooLarge, http.StatusBadRequest), err)
		return
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, _, err = packages_service.CreatePackageAndAddFile(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo:      *pi,
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{Filename: archiveFilename},
			Creator:         ctx.Doer,
			Data:            buf,
			IsLead:          true,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, packages_model.ErrDuplicatePackageVersion):
			apiError(ctx, http.StatusConflict, err)
		case errors.Is(err, packages_service.ErrQuotaTotalCount), errors.Is(err, packages_service.ErrQuotaTypeSize), errors.Is(err, packages_service.ErrQuotaTotalSize):
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

func DeleteModule(ctx *context.Context) {
	pi, err := packageInfo(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	if err := packages_service.RemovePackageVersionByNameAndVersion(ctx, ctx.Doer, pi); err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
