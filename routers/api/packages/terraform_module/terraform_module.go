// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package terraform_module implements the HashiCorp Module Registry
// Protocol on top of Gitea's generic package storage.
//
// See https://developer.hashicorp.com/terraform/internals/module-registry-protocol
//
// Scope of v1:
//   - Only the root module is parsed; submodules and examples are ignored.
//   - Only .tar.gz archives are accepted on upload.
//   - Module versions are immutable: re-uploading the same
//     {namespace, name, provider, version} returns 409 Conflict.
//   - There is no search, no module deprecation; delete is the only
//     mutation other than upload.
package terraform_module

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	packages_model "gitea.dev/models/packages"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	packages_module "gitea.dev/modules/packages"
	tfmod "gitea.dev/modules/packages/terraform_module"
	"gitea.dev/modules/setting"
	"gitea.dev/routers/api/packages/helper"
	"gitea.dev/services/context"
	packages_service "gitea.dev/services/packages"

	"github.com/hashicorp/go-version"
)

// archiveFilename is the canonical filename under which the .tar.gz is stored.
const archiveFilename = "module.tar.gz"

func apiError(ctx *context.Context, status int, obj any) {
	message := helper.ProcessErrorForUser(ctx, status, obj)
	ctx.PlainText(status, message)
}

// NamespaceAssignment resolves the {namespace} path parameter to a Gitea
// user/org and delegates to context.PackageAssignment so ctx.Package
// (owner + access mode) is fully populated for reqPackageAccess and the
// quota helpers. Namespaces are validated against the HashiCorp naming
// rules before lookup so an invalid value yields 400 rather than a 404
// from the user lookup.
func NamespaceAssignment() func(ctx *context.Context) {
	assignPackage := context.PackageAssignment()
	return func(ctx *context.Context) {
		namespace := ctx.PathParam("namespace")
		if err := tfmod.ValidateNamespace(namespace); err != nil {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		owner, err := user_model.GetUserByName(ctx, namespace)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
		ctx.ContextUser = owner
		// PackageAssignment reads ctx.ContextUser, determines the access
		// mode, and skips the version lookup because our routes do not
		// carry the {type} path parameter it would otherwise use.
		assignPackage(ctx)
	}
}

// packageName encodes the registry tuple `{name}/{provider}` into the
// single package name used by the underlying generic package storage.
// Slash is safe because Gitea stores the value verbatim and only
// requires it to be unique per (owner, type).
func packageName(name, provider string) string {
	return fmt.Sprintf("%s/%s", name, provider)
}

// parsePackagePath pulls and validates the {name} and {provider} path
// parameters. Returns 400-friendly errors so handlers can fail fast.
func parsePackagePath(ctx *context.Context) (string, string, error) {
	name := ctx.PathParam("name")
	provider := ctx.PathParam("provider")
	if err := tfmod.ValidateName(name); err != nil {
		return "", "", err
	}
	if err := tfmod.ValidateProvider(provider); err != nil {
		return "", "", err
	}
	return name, provider, nil
}

// ListVersions implements:
//
//	GET :base/:namespace/:name/:provider/versions
//
// Response shape per the spec: an object with a single "modules" array
// whose first element holds the "versions" array.
func ListVersions(ctx *context.Context) {
	name, provider, err := parsePackagePath(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	pkg, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformModule, packageName(name, provider))
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  pkg.ID,
		IsInternal: optional.Some(false),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type versionEntry struct {
		Version string `json:"version"`
	}
	type moduleEntry struct {
		Versions []versionEntry `json:"versions"`
	}
	resp := struct {
		Modules []moduleEntry `json:"modules"`
	}{
		Modules: []moduleEntry{{Versions: make([]versionEntry, 0, len(pvs))}},
	}
	for _, pv := range pvs {
		resp.Modules[0].Versions = append(resp.Modules[0].Versions, versionEntry{Version: pv.Version})
	}
	ctx.JSON(http.StatusOK, resp)
}

// DownloadRedirect implements:
//
//	GET :base/:namespace/:name/:provider/:version/download
//
// Per spec, this returns 204 No Content with an X-Terraform-Get header
// pointing at the archive. We keep the archive on the same authenticated
// path so the credentials Terraform attached to the initial request are
// reused.
func DownloadRedirect(ctx *context.Context) {
	name, provider, err := parsePackagePath(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	v := ctx.PathParam("version")
	if _, err := version.NewSemver(v); err != nil {
		apiError(ctx, http.StatusBadRequest, tfmod.ErrInvalidVersion)
		return
	}

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformModule, packageName(name, provider), v)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	_ = pv // existence is enough; the archive endpoint will re-resolve.

	archiveURL := fmt.Sprintf(
		"%sapi/packages/-/terraform/modules/%s/%s/%s/%s/archive",
		setting.AppURL,
		ctx.Package.Owner.Name, name, provider, v,
	)
	ctx.Resp.Header().Set("X-Terraform-Get", archiveURL)
	ctx.Status(http.StatusNoContent)
}

// DownloadArchive streams the stored .tar.gz blob.
func DownloadArchive(ctx *context.Context) {
	name, provider, err := parsePackagePath(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	v := ctx.PathParam("version")
	if _, err := version.NewSemver(v); err != nil {
		apiError(ctx, http.StatusBadRequest, tfmod.ErrInvalidVersion)
		return
	}

	s, u, pf, err := packages_service.OpenFileForDownloadByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeTerraformModule,
			Name:        packageName(name, provider),
			Version:     v,
		},
		&packages_service.PackageFileInfo{Filename: archiveFilename},
		ctx.Req.Method,
	)
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

// UploadModule implements PUT of a .tar.gz body for a new version.
//
// The endpoint is *not* part of the HashiCorp protocol — every private
// registry invents its own. We accept the archive as the raw request
// body to match how cargo and generic packages publish.
func UploadModule(ctx *context.Context) {
	name, provider, err := parsePackagePath(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	v := ctx.PathParam("version")
	if _, err := version.NewSemver(v); err != nil {
		apiError(ctx, http.StatusBadRequest, tfmod.ErrInvalidVersion)
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
		log.Error("terraform_module: hashed buffer: %v", err)
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	module, err := tfmod.ParseModuleArchive(buf, setting.Packages.LimitSizeTerraformModule)
	if err != nil {
		switch {
		case errors.Is(err, tfmod.ErrArchiveTooLarge):
			apiError(ctx, http.StatusRequestEntityTooLarge, err)
		default:
			apiError(ctx, http.StatusBadRequest, err)
		}
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, _, err = packages_service.CreatePackageAndAddFile(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeTerraformModule,
				Name:        packageName(name, provider),
				Version:     v,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         module.Metadata,
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
		case errors.Is(err, packages_model.ErrDuplicatePackageVersion),
			errors.Is(err, packages_model.ErrDuplicatePackageFile):
			apiError(ctx, http.StatusConflict, err)
		case errors.Is(err, packages_service.ErrQuotaTotalCount),
			errors.Is(err, packages_service.ErrQuotaTypeSize),
			errors.Is(err, packages_service.ErrQuotaTotalSize):
			apiError(ctx, http.StatusForbidden, err)
		default:
			log.Error("terraform_module: create: %v", err)
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

// DeleteModule removes a specific module version. This is the only
// mutation other than upload in v1.
func DeleteModule(ctx *context.Context) {
	name, provider, err := parsePackagePath(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	v := ctx.PathParam("version")
	if _, err := version.NewSemver(v); err != nil {
		apiError(ctx, http.StatusBadRequest, tfmod.ErrInvalidVersion)
		return
	}

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformModule, packageName(name, provider), v)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ServiceDiscovery returns the host-level Terraform service-discovery
// document. Per the spec only the `modules.v1` capability is advertised;
// other capabilities (login.v1, providers.v1, ...) are unimplemented
// and therefore omitted.
func ServiceDiscovery(ctx *context.Context) {
	resp := map[string]string{
		"modules.v1": "/api/packages/-/terraform/modules/",
	}
	ctx.Resp.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(ctx.Resp).Encode(resp)
}
