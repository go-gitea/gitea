// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	nuget_model "code.gitea.io/gitea/models/packages/nuget"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	packages_module "code.gitea.io/gitea/modules/packages"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, map[string]string{
			"Message": message,
		})
	})
}

func xmlResponse(ctx *context.Context, status int, obj any) { //nolint:unparam
	ctx.Resp.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.Resp.WriteHeader(status)
	if _, err := ctx.Resp.Write([]byte(xml.Header)); err != nil {
		log.Error("Write failed: %v", err)
	}
	if err := xml.NewEncoder(ctx.Resp).Encode(obj); err != nil {
		log.Error("XML encode failed: %v", err)
	}
}

// https://github.com/NuGet/NuGet.Client/blob/dev/src/NuGet.Core/NuGet.Protocol/LegacyFeed/V2FeedQueryBuilder.cs
func ServiceIndexV2(ctx *context.Context) {
	base := setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget"

	xmlResponse(ctx, http.StatusOK, &ServiceIndexResponseV2{
		Base:      base,
		Xmlns:     "http://www.w3.org/2007/app",
		XmlnsAtom: "http://www.w3.org/2005/Atom",
		Workspace: ServiceWorkspace{
			Title: AtomTitle{
				Type: "text",
				Text: "Default",
			},
			Collection: ServiceCollection{
				Href: "Packages",
				Title: AtomTitle{
					Type: "text",
					Text: "Packages",
				},
			},
		},
	})
}

// https://docs.microsoft.com/en-us/nuget/api/service-index
func ServiceIndexV3(ctx *context.Context) {
	root := setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget"

	ctx.JSON(http.StatusOK, &ServiceIndexResponseV3{
		Version: "3.0.0",
		Resources: []ServiceResource{
			{ID: root + "/query", Type: "SearchQueryService"},
			{ID: root + "/query", Type: "SearchQueryService/3.0.0-beta"},
			{ID: root + "/query", Type: "SearchQueryService/3.0.0-rc"},
			{ID: root + "/registration", Type: "RegistrationsBaseUrl"},
			{ID: root + "/registration", Type: "RegistrationsBaseUrl/3.0.0-beta"},
			{ID: root + "/registration", Type: "RegistrationsBaseUrl/3.0.0-rc"},
			{ID: root + "/package", Type: "PackageBaseAddress/3.0.0"},
			{ID: root, Type: "PackagePublish/2.0.0"},
			{ID: root + "/symbolpackage", Type: "SymbolPackagePublish/4.9.0"},
		},
	})
}

// https://github.com/NuGet/NuGet.Client/blob/dev/src/NuGet.Core/NuGet.Protocol/LegacyFeed/LegacyFeedCapabilityResourceV2Feed.cs
func FeedCapabilityResource(ctx *context.Context) {
	xmlResponse(ctx, http.StatusOK, Metadata)
}

var (
	searchTermExtract = regexp.MustCompile(`'([^']+)'`)
	searchTermExact   = regexp.MustCompile(`\s+eq\s+'`)
)

func getSearchTerm(ctx *context.Context) packages_model.SearchValue {
	searchTerm := strings.Trim(ctx.FormTrim("searchTerm"), "'")
	if searchTerm != "" {
		return packages_model.SearchValue{
			Value:      searchTerm,
			ExactMatch: false,
		}
	}

	// $filter contains a query like:
	// (((Id ne null) and substringof('microsoft',tolower(Id)))
	// https://www.odata.org/documentation/odata-version-2-0/uri-conventions/ section 4.5
	// We don't support these queries, just extract the search term.
	filter := ctx.FormTrim("$filter")
	match := searchTermExtract.FindStringSubmatch(filter)
	if len(match) == 2 {
		return packages_model.SearchValue{
			Value:      strings.TrimSpace(match[1]),
			ExactMatch: searchTermExact.MatchString(filter),
		}
	}

	return packages_model.SearchValue{}
}

// https://github.com/NuGet/NuGet.Client/blob/dev/src/NuGet.Core/NuGet.Protocol/LegacyFeed/V2FeedQueryBuilder.cs
func SearchServiceV2(ctx *context.Context) {
	skip, take := ctx.FormInt("$skip"), ctx.FormInt("$top")
	paginator := db.NewAbsoluteListOptions(skip, take)

	pvs, total, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeNuGet,
		Name:       getSearchTerm(ctx),
		IsInternal: optional.Some(false),
		Paginator:  paginator,
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

	skip, take = paginator.GetSkipTake()

	var next *nextOptions
	if len(pvs) == take {
		next = &nextOptions{
			Path:  "Search()",
			Query: url.Values{},
		}
		searchTerm := ctx.FormTrim("searchTerm")
		if searchTerm != "" {
			next.Query.Set("searchTerm", searchTerm)
		}
		filter := ctx.FormTrim("$filter")
		if filter != "" {
			next.Query.Set("$filter", filter)
		}
		next.Query.Set("$skip", strconv.Itoa(skip+take))
		next.Query.Set("$top", strconv.Itoa(take))
	}

	resp := createFeedResponse(
		&linkBuilder{Base: setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget", Next: next},
		total,
		pds,
	)

	xmlResponse(ctx, http.StatusOK, resp)
}

// http://docs.oasis-open.org/odata/odata/v4.0/errata03/os/complete/part2-url-conventions/odata-v4.0-errata03-os-part2-url-conventions-complete.html#_Toc453752351
func SearchServiceV2Count(ctx *context.Context) {
	count, err := nuget_model.CountPackages(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Name:       getSearchTerm(ctx),
		IsInternal: optional.Some(false),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusOK, strconv.FormatInt(count, 10))
}

// https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-for-packages
func SearchServiceV3(ctx *context.Context) {
	pvs, count, err := nuget_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Name:       packages_model.SearchValue{Value: ctx.FormTrim("q")},
		IsInternal: optional.Some(false),
		Paginator: db.NewAbsoluteListOptions(
			ctx.FormInt("skip"),
			ctx.FormInt("take"),
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

	resp := createSearchResultResponse(
		&linkBuilder{Base: setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget"},
		count,
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-index
func RegistrationIndex(ctx *context.Context) {
	packageName := ctx.PathParam("id")

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeNuGet, packageName)
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

	resp := createRegistrationIndexResponse(
		&linkBuilder{Base: setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget"},
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// https://github.com/NuGet/NuGet.Client/blob/dev/src/NuGet.Core/NuGet.Protocol/LegacyFeed/V2FeedQueryBuilder.cs
func RegistrationLeafV2(ctx *context.Context) {
	packageName := ctx.PathParam("id")
	packageVersion := ctx.PathParam("version")

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeNuGet, packageName, packageVersion)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
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

	resp := createEntryResponse(
		&linkBuilder{Base: setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget"},
		pd,
	)

	xmlResponse(ctx, http.StatusOK, resp)
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf
func RegistrationLeafV3(ctx *context.Context) {
	packageName := ctx.PathParam("id")
	packageVersion := strings.TrimSuffix(ctx.PathParam("version"), ".json")

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeNuGet, packageName, packageVersion)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
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

	resp := createRegistrationLeafResponse(
		&linkBuilder{Base: setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget"},
		pd,
	)

	ctx.JSON(http.StatusOK, resp)
}

// https://github.com/NuGet/NuGet.Client/blob/dev/src/NuGet.Core/NuGet.Protocol/LegacyFeed/V2FeedQueryBuilder.cs
func EnumeratePackageVersionsV2(ctx *context.Context) {
	packageName := strings.Trim(ctx.FormTrim("id"), "'")

	skip, take := ctx.FormInt("$skip"), ctx.FormInt("$top")
	paginator := db.NewAbsoluteListOptions(skip, take)

	pvs, total, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeNuGet,
		Name: packages_model.SearchValue{
			ExactMatch: true,
			Value:      packageName,
		},
		IsInternal: optional.Some(false),
		Paginator:  paginator,
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

	skip, take = paginator.GetSkipTake()

	var next *nextOptions
	if len(pvs) == take {
		next = &nextOptions{
			Path:  "FindPackagesById()",
			Query: url.Values{},
		}
		next.Query.Set("id", packageName)
		next.Query.Set("$skip", strconv.Itoa(skip+take))
		next.Query.Set("$top", strconv.Itoa(take))
	}

	resp := createFeedResponse(
		&linkBuilder{Base: setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/nuget", Next: next},
		total,
		pds,
	)

	xmlResponse(ctx, http.StatusOK, resp)
}

// http://docs.oasis-open.org/odata/odata/v4.0/errata03/os/complete/part2-url-conventions/odata-v4.0-errata03-os-part2-url-conventions-complete.html#_Toc453752351
func EnumeratePackageVersionsV2Count(ctx *context.Context) {
	count, err := packages_model.CountVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeNuGet,
		Name: packages_model.SearchValue{
			ExactMatch: true,
			Value:      strings.Trim(ctx.FormTrim("id"), "'"),
		},
		IsInternal: optional.Some(false),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusOK, strconv.FormatInt(count, 10))
}

// https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#enumerate-package-versions
func EnumeratePackageVersionsV3(ctx *context.Context) {
	packageName := ctx.PathParam("id")

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeNuGet, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	resp := createPackageVersionsResponse(pvs)

	ctx.JSON(http.StatusOK, resp)
}

// https://learn.microsoft.com/en-us/nuget/api/package-base-address-resource#download-package-manifest-nuspec
// https://learn.microsoft.com/en-us/nuget/api/package-base-address-resource#download-package-content-nupkg
func DownloadPackageFile(ctx *context.Context) {
	packageName := ctx.PathParam("id")
	packageVersion := ctx.PathParam("version")
	filename := ctx.PathParam("filename")

	s, u, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeNuGet,
			Name:        packageName,
			Version:     packageVersion,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
		},
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

// UploadPackage creates a new package with the metadata contained in the uploaded nupgk file
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#push-a-package
func UploadPackage(ctx *context.Context) {
	np, buf, closables := processUploadedFile(ctx, nuget_module.DependencyPackage)
	defer func() {
		for _, c := range closables {
			c.Close()
		}
	}()
	if np == nil {
		return
	}

	pv, _, err := packages_service.CreatePackageAndAddFile(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeNuGet,
				Name:        np.ID,
				Version:     np.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         np.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s.%s.nupkg", np.ID, np.Version)),
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

	nuspecBuf, err := packages_module.CreateHashedBufferFromReaderWithSize(np.NuspecContent, np.NuspecContent.Len())
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer nuspecBuf.Close()

	_, err = packages_service.AddFileToPackageVersionInternal(
		ctx,
		pv,
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s.nuspec", np.ID)),
			},
			Data: nuspecBuf,
		},
	)
	if err != nil {
		switch err {
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

// UploadSymbolPackage adds a symbol package to an existing package
// https://docs.microsoft.com/en-us/nuget/api/symbol-package-publish-resource
func UploadSymbolPackage(ctx *context.Context) {
	np, buf, closables := processUploadedFile(ctx, nuget_module.SymbolsPackage)
	defer func() {
		for _, c := range closables {
			c.Close()
		}
	}()
	if np == nil {
		return
	}

	pdbs, err := nuget_module.ExtractPortablePdb(buf, buf.Size())
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer pdbs.Close()

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pi := &packages_service.PackageInfo{
		Owner:       ctx.Package.Owner,
		PackageType: packages_model.TypeNuGet,
		Name:        np.ID,
		Version:     np.Version,
	}

	_, err = packages_service.AddFileToExistingPackage(
		ctx,
		pi,
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s.%s.snupkg", np.ID, np.Version)),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  false,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrPackageNotExist:
			apiError(ctx, http.StatusNotFound, err)
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	for _, pdb := range pdbs {
		_, err := packages_service.AddFileToExistingPackage(
			ctx,
			pi,
			&packages_service.PackageFileCreationInfo{
				PackageFileInfo: packages_service.PackageFileInfo{
					Filename:     strings.ToLower(pdb.Name),
					CompositeKey: strings.ToLower(pdb.ID),
				},
				Creator: ctx.Doer,
				Data:    pdb.Content,
				IsLead:  false,
				Properties: map[string]string{
					nuget_module.PropertySymbolID: strings.ToLower(pdb.ID),
				},
			},
		)
		if err != nil {
			switch err {
			case packages_model.ErrDuplicatePackageFile:
				apiError(ctx, http.StatusConflict, err)
			case packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
				apiError(ctx, http.StatusForbidden, err)
			default:
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
	}

	ctx.Status(http.StatusCreated)
}

func processUploadedFile(ctx *context.Context, expectedType nuget_module.PackageType) (*nuget_module.Package, *packages_module.HashedBuffer, []io.Closer) {
	closables := make([]io.Closer, 0, 2)

	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return nil, nil, closables
	}

	if needToClose {
		closables = append(closables, upload)
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return nil, nil, closables
	}
	closables = append(closables, buf)

	np, err := nuget_module.ParsePackageMetaData(buf, buf.Size())
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
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

// https://github.com/dotnet/symstore/blob/main/docs/specs/Simple_Symbol_Query_Protocol.md#request
func DownloadSymbolFile(ctx *context.Context) {
	filename := ctx.PathParam("filename")
	guid := ctx.PathParam("guid")[:32]
	filename2 := ctx.PathParam("filename2")

	if filename != filename2 {
		apiError(ctx, http.StatusBadRequest, nil)
		return
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:     ctx.Package.Owner.ID,
		PackageType: packages_model.TypeNuGet,
		Query:       filename,
		Properties: map[string]string{
			nuget_module.PropertySymbolID: strings.ToLower(guid),
		},
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	s, u, pf, err := packages_service.GetPackageFileStream(ctx, pfs[0])
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

// DeletePackage hard deletes the package
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#delete-a-package
func DeletePackage(ctx *context.Context) {
	packageName := ctx.PathParam("id")
	packageVersion := ctx.PathParam("version")

	err := packages_service.RemovePackageVersionByNameAndVersion(
		ctx,
		ctx.Doer,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeNuGet,
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
	}

	ctx.Status(http.StatusNoContent)
}
