// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	npm_module "code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"
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

	ctx.ServeStream(s, pf.Name)
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
		IsInternal:      false,
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
	npmPackage, err := npm_module.ParsePackage(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
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
			Data:   buf,
			IsLead: true,
		},
	)
	if err != nil {
		if err == packages_model.ErrDuplicatePackageVersion {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
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

	ctx.Status(http.StatusCreated)
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

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID: pv.PackageID,
		Properties: map[string]string{
			npm_module.TagProperty: tag,
		},
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
