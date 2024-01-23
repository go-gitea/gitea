// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swift

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	swift_module "code.gitea.io/gitea/modules/packages/swift"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#35-api-versioning
const (
	AcceptJSON  = "application/vnd.swift.registry.v1+json"
	AcceptSwift = "application/vnd.swift.registry.v1+swift"
	AcceptZip   = "application/vnd.swift.registry.v1+zip"
)

var (
	// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#361-package-scope
	scopePattern = regexp.MustCompile(`\A[a-zA-Z0-9][a-zA-Z0-9-]{0,38}\z`)
	// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#362-package-name
	namePattern = regexp.MustCompile(`\A[a-zA-Z0-9][a-zA-Z0-9-_]{0,99}\z`)
)

type headers struct {
	Status      int
	ContentType string
	Digest      string
	Location    string
	Link        string
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#35-api-versioning
func setResponseHeaders(resp http.ResponseWriter, h *headers) {
	if h.ContentType != "" {
		resp.Header().Set("Content-Type", h.ContentType)
	}
	if h.Digest != "" {
		resp.Header().Set("Digest", "sha256="+h.Digest)
	}
	if h.Location != "" {
		resp.Header().Set("Location", h.Location)
	}
	if h.Link != "" {
		resp.Header().Set("Link", h.Link)
	}
	resp.Header().Set("Content-Version", "1")
	if h.Status != 0 {
		resp.WriteHeader(h.Status)
	}
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#33-error-handling
func apiError(ctx *context.Context, status int, obj any) {
	// https://www.rfc-editor.org/rfc/rfc7807
	type Problem struct {
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}

	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		setResponseHeaders(ctx.Resp, &headers{
			Status:      status,
			ContentType: "application/problem+json",
		})
		if err := json.NewEncoder(ctx.Resp).Encode(Problem{
			Status: status,
			Detail: message,
		}); err != nil {
			log.Error("JSON encode: %v", err)
		}
	})
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#35-api-versioning
func CheckAcceptMediaType(requiredAcceptHeader string) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		accept := ctx.Req.Header.Get("Accept")
		if accept != "" && accept != requiredAcceptHeader {
			apiError(ctx, http.StatusBadRequest, fmt.Sprintf("Unexpected accept header. Should be '%s'.", requiredAcceptHeader))
		}
	}
}

func buildPackageID(scope, name string) string {
	return scope + "." + name
}

type Release struct {
	URL string `json:"url"`
}

type EnumeratePackageVersionsResponse struct {
	Releases map[string]Release `json:"releases"`
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#41-list-package-releases
func EnumeratePackageVersions(ctx *context.Context) {
	packageScope := ctx.Params("scope")
	packageName := ctx.Params("name")

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeSwift, buildPackageID(packageScope, packageName))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	sort.Slice(pds, func(i, j int) bool {
		return pds[i].SemVer.LessThan(pds[j].SemVer)
	})

	baseURL := fmt.Sprintf("%sapi/packages/%s/swift/%s/%s/", setting.AppURL, ctx.Package.Owner.LowerName, packageScope, packageName)

	releases := make(map[string]Release)
	for _, pd := range pds {
		version := pd.SemVer.String()
		releases[version] = Release{
			URL: baseURL + version,
		}
	}

	setResponseHeaders(ctx.Resp, &headers{
		Link: fmt.Sprintf(`<%s%s>; rel="latest-version"`, baseURL, pds[len(pds)-1].Version.Version),
	})

	ctx.JSON(http.StatusOK, EnumeratePackageVersionsResponse{
		Releases: releases,
	})
}

type Resource struct {
	Name     string `json:"id"`
	Type     string `json:"type"`
	Checksum string `json:"checksum"`
}

type PackageVersionMetadataResponse struct {
	ID        string                           `json:"id"`
	Version   string                           `json:"version"`
	Resources []Resource                       `json:"resources"`
	Metadata  *swift_module.SoftwareSourceCode `json:"metadata"`
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#endpoint-2
func PackageVersionMetadata(ctx *context.Context) {
	id := buildPackageID(ctx.Params("scope"), ctx.Params("name"))

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeSwift, id, ctx.Params("version"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	metadata := pd.Metadata.(*swift_module.Metadata)

	setResponseHeaders(ctx.Resp, &headers{})

	ctx.JSON(http.StatusOK, PackageVersionMetadataResponse{
		ID:      id,
		Version: pd.Version.Version,
		Resources: []Resource{
			{
				Name:     "source-archive",
				Type:     "application/zip",
				Checksum: pd.Files[0].Blob.HashSHA256,
			},
		},
		Metadata: &swift_module.SoftwareSourceCode{
			Context:        []string{"http://schema.org/"},
			Type:           "SoftwareSourceCode",
			Name:           pd.PackageProperties.GetByName(swift_module.PropertyName),
			Version:        pd.Version.Version,
			Description:    metadata.Description,
			Keywords:       metadata.Keywords,
			CodeRepository: metadata.RepositoryURL,
			License:        metadata.License,
			ProgrammingLanguage: swift_module.ProgrammingLanguage{
				Type: "ComputerLanguage",
				Name: "Swift",
				URL:  "https://swift.org",
			},
			Author: swift_module.Person{
				Type:       "Person",
				GivenName:  metadata.Author.GivenName,
				MiddleName: metadata.Author.MiddleName,
				FamilyName: metadata.Author.FamilyName,
			},
		},
	})
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#43-fetch-manifest-for-a-package-release
func DownloadManifest(ctx *context.Context) {
	packageScope := ctx.Params("scope")
	packageName := ctx.Params("name")
	packageVersion := ctx.Params("version")

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeSwift, buildPackageID(packageScope, packageName), packageVersion)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	swiftVersion := ctx.FormTrim("swift-version")
	if swiftVersion != "" {
		v, err := version.NewVersion(swiftVersion)
		if err == nil {
			swiftVersion = swift_module.TrimmedVersionString(v)
		}
	}
	m, ok := pd.Metadata.(*swift_module.Metadata).Manifests[swiftVersion]
	if !ok {
		setResponseHeaders(ctx.Resp, &headers{
			Status:   http.StatusSeeOther,
			Location: fmt.Sprintf("%sapi/packages/%s/swift/%s/%s/%s/Package.swift", setting.AppURL, ctx.Package.Owner.LowerName, packageScope, packageName, packageVersion),
		})
		return
	}

	setResponseHeaders(ctx.Resp, &headers{})

	filename := "Package.swift"
	if swiftVersion != "" {
		filename = fmt.Sprintf("Package@swift-%s.swift", swiftVersion)
	}

	ctx.ServeContent(strings.NewReader(m.Content), &context.ServeHeaderOptions{
		ContentType:  "text/x-swift",
		Filename:     filename,
		LastModified: pv.CreatedUnix.AsLocalTime(),
	})
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#endpoint-6
func UploadPackageFile(ctx *context.Context) {
	packageScope := ctx.Params("scope")
	packageName := ctx.Params("name")

	v, err := version.NewVersion(ctx.Params("version"))

	if !scopePattern.MatchString(packageScope) || !namePattern.MatchString(packageName) || err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	packageVersion := v.Core().String()

	file, _, err := ctx.Req.FormFile("source-archive")
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	buf, err := packages_module.CreateHashedBufferFromReader(file)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	var mr io.Reader
	metadata := ctx.Req.FormValue("metadata")
	if metadata != "" {
		mr = strings.NewReader(metadata)
	}

	pck, err := swift_module.ParsePackage(buf, buf.Size(), mr)
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

	pv, _, err := packages_service.CreatePackageAndAddFile(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeSwift,
				Name:        buildPackageID(packageScope, packageName),
				Version:     packageVersion,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         pck.Metadata,
			PackageProperties: map[string]string{
				swift_module.PropertyScope: packageScope,
				swift_module.PropertyName:  packageName,
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: fmt.Sprintf("%s-%s.zip", packageName, packageVersion),
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

	for _, url := range pck.RepositoryURLs {
		_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, swift_module.PropertyRepositoryURL, url)
		if err != nil {
			log.Error("InsertProperty failed: %v", err)
		}
	}

	setResponseHeaders(ctx.Resp, &headers{})

	ctx.Status(http.StatusCreated)
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#endpoint-4
func DownloadPackageFile(ctx *context.Context) {
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeSwift, buildPackageID(ctx.Params("scope"), ctx.Params("name")), ctx.Params("version"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pf := pd.Files[0].File

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	setResponseHeaders(ctx.Resp, &headers{
		Digest: pd.Files[0].Blob.HashSHA256,
	})

	helper.ServePackageFile(ctx, s, u, pf, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		ContentType:  "application/zip",
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

type LookupPackageIdentifiersResponse struct {
	Identifiers []string `json:"identifiers"`
}

// https://github.com/apple/swift-package-manager/blob/main/Documentation/Registry.md#endpoint-5
func LookupPackageIdentifiers(ctx *context.Context) {
	url := ctx.FormTrim("url")
	if url == "" {
		apiError(ctx, http.StatusBadRequest, nil)
		return
	}

	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeSwift,
		Properties: map[string]string{
			swift_module.PropertyRepositoryURL: url,
		},
		IsInternal: util.OptionalBoolFalse,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	identifiers := make([]string, 0, len(pds))
	for _, pd := range pds {
		identifiers = append(identifiers, pd.Package.Name)
	}

	setResponseHeaders(ctx.Resp, &headers{})

	ctx.JSON(http.StatusOK, LookupPackageIdentifiersResponse{
		Identifiers: identifiers,
	})
}
