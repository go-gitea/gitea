// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	npm_module "code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

// errInvalidTagName indicates an invalid tag name
var errInvalidTagName = errors.New("The tag name is invalid")

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, map[string]string{
			"error": message,
		})
	})
}

// packageNameFromParams gets the package name from the url parameters
// Variations: /name/, /@scope/name/, /@scope%2Fname/
func packageNameFromParams(ctx *context.Context) string {
	scope := ctx.Params("scope")
	id := ctx.Params("id")
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
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
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

// DownloadPackageFileByName finds the version and serves the contents of a package
func DownloadPackageFileByName(ctx *context.Context) {
	filename := ctx.Params("filename")

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeNpm,
		Name: packages_model.SearchValue{
			ExactMatch: true,
			Value:      packageNameFromParams(ctx),
		},
		HasFileWithName: filename,
		IsInternal:      util.OptionalBoolFalse,
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

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

// UploadPackage creates a new package
func UploadPackage(ctx *context.Context) {
	npmPackage, err := npm_module.ParsePackage(ctx.Req.Body)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	repo, err := repo_model.GetRepositoryByURL(ctx, npmPackage.Metadata.Repository.URL)
	if err == nil {
		canWrite := repo.OwnerID == ctx.Doer.ID

		if !canWrite {
			perms, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
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

	buf, err := packages_module.CreateHashedBufferFromReader(bytes.NewReader(npmPackage.Data), 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pv, _, err := packages_service.CreatePackageAndAddFile(
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
			apiError(ctx, http.StatusBadRequest, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	for _, tag := range npmPackage.DistTags {
		if err := setPackageTag(tag, pv, false); err != nil {
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

// DeletePackageVersion deletes the package version
func DeletePackageVersion(ctx *context.Context) {
	packageName := packageNameFromParams(ctx)
	packageVersion := ctx.Params("version")

	err := packages_service.RemovePackageVersionByNameAndVersion(
		ctx.Doer,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeNpm,
			Name:        packageName,
			Version:     packageVersion,
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
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
		if err := packages_service.RemovePackageVersion(ctx.Doer, pv); err != nil {
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

	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	version := strings.Trim(string(body), "\"") // is as "version" in the body

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeNpm, packageName, version)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := setPackageTag(ctx.Params("tag"), pv, false); err != nil {
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
		if err := setPackageTag(ctx.Params("tag"), pvs[0], true); err != nil {
			if err == errInvalidTagName {
				apiError(ctx, http.StatusBadRequest, err)
				return
			}
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}
}

func setPackageTag(tag string, pv *packages_model.PackageVersion, deleteOnly bool) error {
	if tag == "" {
		return errInvalidTagName
	}
	_, err := version.NewVersion(tag)
	if err == nil {
		return errInvalidTagName
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID: pv.PackageID,
		Properties: map[string]string{
			npm_module.TagProperty: tag,
		},
		IsInternal: util.OptionalBoolFalse,
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

	return committer.Commit()
}

func PackageSearch(ctx *context.Context) {
	pvs, total, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeNpm,
		IsInternal: util.OptionalBoolFalse,
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
