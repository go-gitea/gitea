// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"bytes"
	std_ctx "context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitea.dev/models/db"
	packages_model "gitea.dev/models/packages"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/json"
	"gitea.dev/modules/optional"
	packages_module "gitea.dev/modules/packages"
	npm_module "gitea.dev/modules/packages/npm"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/packages/helper"
	"gitea.dev/services/context"
	packages_service "gitea.dev/services/packages"

	"github.com/hashicorp/go-version"
)

// errInvalidTagName indicates an invalid tag name
var errInvalidTagName = errors.New("The tag name is invalid")

// maxNpmUploadBodyFallback caps the buffered publish body when no explicit
// npm size limit is configured. npm tarballs are base64-encoded inside the
// JSON envelope, so a hard ceiling here prevents an unbounded
// io.ReadAll on the request body.
const maxNpmUploadBodyFallback = int64(1 << 30) // 1 GiB

// npmUploadBodyLimit returns the maximum number of bytes UploadPackage will
// buffer from the request body. When LIMIT_SIZE_NPM is set, allow ~2x the
// tarball size to account for base64 expansion (~33%) plus JSON envelope and
// metadata overhead; otherwise fall back to a generous hard ceiling.
func npmUploadBodyLimit() int64 {
	if l := setting.Packages.LimitSizeNpm; l > 0 {
		return l*2 + 64*1024
	}
	return maxNpmUploadBodyFallback
}

func apiError(ctx *context.Context, status int, obj any) {
	message := helper.ProcessErrorForUser(ctx, status, obj)
	ctx.JSON(status, map[string]string{
		"error": message,
	})
}

// packageNameFromParams gets the package name from the url parameters
// Variations: /name/, /@scope/name/, /@scope%2Fname/
func packageNameFromParams(ctx *context.Context) string {
	scope := ctx.PathParam("scope")
	id := ctx.PathParam("id")
	if scope != "" {
		return fmt.Sprintf("@%s/%s", scope, id)
	}
	return id
}

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, packageName)
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

	resp := createPackageMetadataResponse(
		setting.AppURL+"api/packages/"+ctx.Package.Owner.Name+"/npm",
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)
	packageVersion := ctx.PathParam("version")
	filename := ctx.PathParam("filename")

	s, u, pf, err := packages_service.OpenFileForDownloadByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeNpm,
			Name:        packageName,
			Version:     packageVersion,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
		},
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

// DownloadPackageFileByName finds the version and serves the contents of a package
func DownloadPackageFileByName(ctx *context.Context) {
	filename := ctx.PathParam("filename")

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeNpm,
		Name: packages_model.SearchValue{
			ExactMatch: true,
			Value:      packageNameFromParams(ctx),
		},
		HasFileWithName: filename,
		IsInternal:      optional.Some(false),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	s, u, pf, err := packages_service.OpenFileForDownloadByPackageVersion(
		ctx,
		pvs[0],
		&packages_service.PackageFileInfo{
			Filename: filename,
		},
		ctx.Req.Method,
	)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageFileNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

// UploadPackage creates a new package
func UploadPackage(ctx *context.Context) {
	limit := npmUploadBodyLimit()
	lr := &io.LimitedReader{R: ctx.Req.Body, N: limit + 1}
	body, err := io.ReadAll(lr)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if int64(len(body)) > limit {
		apiError(ctx, http.StatusRequestEntityTooLarge, "npm publish payload exceeds size limit")
		return
	}

	// `npm deprecate` reuses the same PUT endpoint, but sends the package
	// document with no `_attachments`. Detect that case and update the
	// stored metadata instead of creating a new version.
	if npm_module.IsDeprecateRequest(body) {
		deprecatePackage(ctx, body)
		return
	}

	npmPackage, err := npm_module.ParsePackage(bytes.NewReader(body))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	repo, err := repo_model.GetRepositoryByURLRelax(ctx, npmPackage.Metadata.Repository.URL)
	if err == nil {
		canWrite := repo.OwnerID == ctx.Doer.ID

		if !canWrite {
			perms, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}

			canWrite = perms.CanWrite(unit.TypePackages)
		}

		if !canWrite {
			apiError(ctx, http.StatusForbidden, "no permission to upload this package")
			return
		}
	}

	buf, err := packages_module.CreateHashedBufferFromReader(bytes.NewReader(npmPackage.Data))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pv, _, err := packages_service.CreatePackageAndAddFile(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeNpm,
				Name:        npmPackage.Name,
				Version:     npmPackage.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         npmPackage.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: npmPackage.Filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	for _, tag := range npmPackage.DistTags {
		if err := setPackageTag(ctx, tag, pv, false); err != nil {
			if err == errInvalidTagName {
				apiError(ctx, http.StatusBadRequest, err)
				return
			}
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	if repo != nil {
		if err := packages_model.SetRepositoryLink(ctx, pv.PackageID, repo.ID); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.Status(http.StatusCreated)
}

// DeletePreview does nothing
// The client tells the server what package version it knows about after deleting a version.
func DeletePreview(ctx *context.Context) {
	ctx.Status(http.StatusOK)
}

// deprecatePackage handles an `npm deprecate` request, which is a PUT to the
// package URL with no attachments and a `deprecated` string set on each
// affected version (empty string means undeprecate).
func deprecatePackage(ctx *context.Context, body []byte) {
	dep, err := npm_module.ParsePackageDeprecation(bytes.NewReader(body))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	for version, message := range dep.Versions {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, dep.PackageName, version)
		if err != nil {
			if errors.Is(err, packages_model.ErrPackageNotExist) {
				continue
			}
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		metadata := &npm_module.Metadata{}
		if err := json.Unmarshal([]byte(pv.MetadataJSON), metadata); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		if metadata.Deprecated == message {
			continue
		}
		metadata.Deprecated = message

		raw, err := json.Marshal(metadata)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		pv.MetadataJSON = string(raw)

		if err := packages_model.UpdateVersion(ctx, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.Status(http.StatusOK)
}

// DeletePackageVersion deletes the package version
func DeletePackageVersion(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)
	packageVersion := ctx.PathParam("version")

	err := packages_service.RemovePackageVersionByNameAndVersion(
		ctx,
		ctx.Doer,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeNpm,
			Name:        packageName,
			Version:     packageVersion,
		},
	)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// DeletePackage deletes the package and all versions
func DeletePackage(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	for _, pv := range pvs {
		if err := packages_service.RemovePackageVersion(ctx, ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.Status(http.StatusOK)
}

// ListPackageTags returns all tags for a package
func ListPackageTags(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	tags := make(map[string]string)
	for _, pv := range pvs {
		pvps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeVersion, pv.ID, npm_module.TagProperty)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		for _, pvp := range pvps {
			tags[pvp.Value] = pv.Version
		}
	}

	ctx.JSON(http.StatusOK, tags)
}

// AddPackageTag adds a tag to the package
func AddPackageTag(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)

	// the dist-tag body is only a quoted version string; bound it to avoid an unbounded
	// read that could exhaust memory
	const maxDistTagBodySize = 4 * 1024
	body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, maxDistTagBodySize+1))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(body) > maxDistTagBodySize {
		apiError(ctx, http.StatusRequestEntityTooLarge, errors.New("request body too large"))
		return
	}
	version := strings.Trim(string(body), "\"") // is as "version" in the body

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, packageName, version)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := setPackageTag(ctx, ctx.PathParam("tag"), pv, false); err != nil {
		if err == errInvalidTagName {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
}

// DeletePackageTag deletes a package tag
func DeletePackageTag(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) != 0 {
		if err := setPackageTag(ctx, ctx.PathParam("tag"), pvs[0], true); err != nil {
			if err == errInvalidTagName {
				apiError(ctx, http.StatusBadRequest, err)
				return
			}
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}
}

func setPackageTag(ctx std_ctx.Context, tag string, pv *packages_model.PackageVersion, deleteOnly bool) error {
	if tag == "" {
		return errInvalidTagName
	}
	_, err := version.NewVersion(tag)
	if err == nil {
		return errInvalidTagName
	}

	return db.WithTx(ctx, func(ctx std_ctx.Context) error {
		pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
			PackageID: pv.PackageID,
			Properties: map[string]string{
				npm_module.TagProperty: tag,
			},
			IsInternal: optional.Some(false),
		})
		if err != nil {
			return err
		}

		if len(pvs) == 1 {
			pvps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeVersion, pvs[0].ID, npm_module.TagProperty)
			if err != nil {
				return err
			}

			for _, pvp := range pvps {
				if pvp.Value == tag {
					if err := packages_model.DeletePropertyByID(ctx, pvp.ID); err != nil {
						return err
					}
					break
				}
			}
		}

		if !deleteOnly {
			_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, npm_module.TagProperty, tag)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func PackageSearch(ctx *context.Context) {
	pvs, total, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeNpm,
		IsInternal: optional.Some(false),
		Name: packages_model.SearchValue{
			ExactMatch: false,
			Value:      ctx.FormTrim("text"),
		},
		Paginator: db.NewAbsoluteListOptions(
			ctx.FormInt("from"),
			ctx.FormInt("size"),
		),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createPackageSearchResponse(
		pds,
		total,
	)

	ctx.JSON(http.StatusOK, resp)
}
