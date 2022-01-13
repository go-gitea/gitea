// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	package_module "code.gitea.io/gitea/modules/packages"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/setting"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	packages_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, map[string]string{
			"Message": message,
		})
	})
}

// ServiceIndex https://docs.microsoft.com/en-us/nuget/api/service-index
func ServiceIndex(ctx *context.APIContext) {
	resp := createServiceIndexResponse(setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/nuget")

	ctx.JSON(http.StatusOK, resp)
}

// SearchService https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-for-packages
func SearchService(ctx *context.APIContext) {
	pvs, count, err := packages.SearchVersions(&packages.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    string(packages.TypeNuGet),
		Query:   ctx.FormTrim("q"),
		Paginator: db.NewAbsoluteListOptions(
			ctx.FormInt("skip"),
			ctx.FormInt("take"),
		),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createSearchResultResponse(
		&linkBuilder{setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/nuget"},
		count,
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// RegistrationIndex https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-index
func RegistrationIndex(ctx *context.APIContext) {
	packageName := ctx.Params("id")

	pvs, err := packages.GetVersionsByPackageName(ctx.Package.Owner.ID, packages.TypeNuGet, packageName)
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

	resp := createRegistrationIndexResponse(
		&linkBuilder{setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/nuget"},
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// RegistrationLeaf https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf
func RegistrationLeaf(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := strings.TrimSuffix(ctx.Params("version"), ".json")

	pv, err := packages.GetVersionByNameAndVersion(db.DefaultContext, ctx.Package.Owner.ID, packages.TypeNuGet, packageName, packageVersion, packages.EmptyVersionKey)
	if err != nil {
		if err == packages.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pd, err := packages.GetPackageDescriptor(pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createRegistrationLeafResponse(
		&linkBuilder{setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/nuget"},
		pd,
	)

	ctx.JSON(http.StatusOK, resp)
}

// EnumeratePackageVersions https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#enumerate-package-versions
func EnumeratePackageVersions(ctx *context.APIContext) {
	packageName := ctx.Params("id")

	pvs, err := packages.GetVersionsByPackageName(ctx.Package.Owner.ID, packages.TypeNuGet, packageName)
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

	resp := createPackageVersionsResponse(pds)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#download-package-content-nupkg
func DownloadPackageFile(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages.TypeNuGet,
			Name:        packageName,
			Version:     packageVersion,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
		},
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

// UploadPackage creates a new package with the metadata contained in the uploaded nupgk file
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#push-a-package
func UploadPackage(ctx *context.APIContext) {
	np, buf, closables := processUploadedFile(ctx, nuget_module.DependencyPackage)
	defer func() {
		for _, c := range closables {
			c.Close()
		}
	}()
	if np == nil {
		return
	}

	_, _, err := packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages.TypeNuGet,
				Name:        np.ID,
				Version:     np.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.User,
			Metadata:         np.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s.%s.nupkg", np.ID, np.Version)),
			},
			Data:   buf,
			IsLead: true,
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

	ctx.Status(http.StatusCreated)
}

// UploadSymbolPackage adds a symbol package to an existing package
// https://docs.microsoft.com/en-us/nuget/api/symbol-package-publish-resource
func UploadSymbolPackage(ctx *context.APIContext) {
	np, buf, closables := processUploadedFile(ctx, nuget_module.SymbolsPackage)
	defer func() {
		for _, c := range closables {
			c.Close()
		}
	}()
	if np == nil {
		return
	}

	_, _, err := packages_service.AddFileToExistingPackage(
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages.TypeNuGet,
			Name:        np.ID,
			Version:     np.Version,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s.%s.snupkg", np.ID, np.Version)),
			},
			Data:   buf,
			IsLead: false,
		},
	)
	if err != nil {
		switch err {
		case packages.ErrPackageNotExist:
			apiError(ctx, http.StatusNotFound, err)
		case packages.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusBadRequest, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

func processUploadedFile(ctx *context.APIContext, expectedType nuget_module.PackageType) (*nuget_module.Package, *package_module.HashedBuffer, []io.Closer) {
	closables := make([]io.Closer, 0, 2)

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return nil, nil, closables
	}

	if close {
		closables = append(closables, upload)
	}

	buf, err := package_module.CreateHashedBufferFromReader(upload, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return nil, nil, closables
	}
	closables = append(closables, buf)

	np, err := nuget_module.ParsePackageMetaData(buf, buf.Size())
	if err != nil {
		if err == nuget_module.ErrMissingNuspecFile || err == nuget_module.ErrNuspecFileTooLarge || err == nuget_module.ErrNuspecInvalidID || err == nuget_module.ErrNuspecInvalidVersion {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return nil, nil, closables
	}
	if np.PackageType != expectedType {
		apiError(ctx, http.StatusBadRequest, errors.New("unexpected package type"))
		return nil, nil, closables
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return nil, nil, closables
	}
	return np, buf, closables
}

// DeletePackage hard deletes the package
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#delete-a-package
func DeletePackage(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")

	err := packages_service.DeletePackageVersionByNameAndVersion(
		ctx.User,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages.TypeNuGet,
			Name:        packageName,
			Version:     packageVersion,
		},
	)
	if err != nil {
		if err == packages.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
	}
}
