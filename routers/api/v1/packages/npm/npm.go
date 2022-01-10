// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package npm

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	package_module "code.gitea.io/gitea/modules/packages"
	npm_module "code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

var (
	// errInvalidTagName indicates an invalid tag name
	errInvalidTagName = errors.New("The tag name is invalid")
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, map[string]string{
			"error": message,
		})
	})
}

// packageNameFromParams gets the package name from the url parameters
// Variations: /name/, /@scope/name/, /@scope%2Fname/
func packageNameFromParams(ctx *context.APIContext) (string, error) {
	scope := ctx.Params("scope")
	id := ctx.Params("id")
	if scope != "" {
		return fmt.Sprintf("@%s/%s", scope, id), nil
	}
	return url.QueryUnescape(id)
}

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.APIContext) {
	packageName, err := packageNameFromParams(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	pvs, err := packages.GetVersionsByPackageName(ctx.Package.Owner.ID, packages.TypeNpm, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createPackageMetadataResponse(
		setting.AppURL+"api/v1/packages/"+ctx.Package.Owner.Name+"/npm",
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	packageName, err := packageNameFromParams(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages.TypeNpm,
			Name:        packageName,
			Version:     packageVersion,
		},
		filename,
	)
	if err != nil {
		if err == packages.ErrPackageNotExist || err == packages.ErrPackageFileNotExist {
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
func UploadPackage(ctx *context.APIContext) {
	npmPackage, err := npm_module.ParsePackage(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	buf, err := package_module.CreateHashedBufferFromReader(bytes.NewReader(npmPackage.Data), 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pv, _, err := packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages.TypeNpm,
				Name:        npmPackage.Name,
				Version:     npmPackage.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.User,
			Metadata:         npmPackage.Metadata,
		},
		&packages_service.PackageFileInfo{
			Filename: npmPackage.Filename,
			Data:     buf,
			IsLead:   true,
		},
	)
	if err != nil {
		if err == packages.ErrDuplicatePackageVersion {
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
func ListPackageTags(ctx *context.APIContext) {
	packageName, err := packageNameFromParams(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	pvs, err := packages.GetVersionsByPackageName(ctx.Package.Owner.ID, packages.TypeNpm, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	tags := make(map[string]string)
	for _, pv := range pvs {
		pvps, err := packages.GetVersionPropertiesByName(ctx, pv.ID, npm_module.TagProperty)
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
func AddPackageTag(ctx *context.APIContext) {
	packageName, err := packageNameFromParams(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	body, err := ioutil.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	version := strings.Trim(string(body), "\"") // is as "version" in the body

	pv, err := packages.GetVersionByNameAndVersion(ctx, ctx.ContextUser.ID, packages.TypeNpm, packageName, version)
	if err != nil {
		if err == packages.ErrPackageNotExist {
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
func DeletePackageTag(ctx *context.APIContext) {
	packageName, err := packageNameFromParams(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	pvs, err := packages.GetVersionsByPackageName(ctx.ContextUser.ID, packages.TypeNpm, packageName)
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

func setPackageTag(tag string, pv *packages.PackageVersion, deleteOnly bool) error {
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

	pvs, err := packages.FindVersionsByPropertyNameAndValue(ctx, pv.PackageID, npm_module.TagProperty, tag)
	if err != nil {
		return err
	}

	if len(pvs) == 1 {
		pvps, err := packages.GetVersionPropertiesByName(ctx, pvs[0].ID, npm_module.TagProperty)
		if err != nil {
			return err
		}

		for _, pvp := range pvps {
			if pvp.Value == tag {
				if err := packages.DeleteVersionPropertyByID(ctx, pvp.ID); err != nil {
					return err
				}
				break
			}
		}
	}

	if !deleteOnly {
		_, err = packages.InsertVersionProperty(ctx, pv.ID, npm_module.TagProperty, tag)
		if err != nil {
			return err
		}
	}

	return committer.Commit()
}
