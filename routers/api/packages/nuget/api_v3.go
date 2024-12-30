// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"sort"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// https://docs.microsoft.com/en-us/nuget/api/service-index#resources
type ServiceIndexResponseV3 struct {
	Version   string            `json:"version"`
	Resources []ServiceResource `json:"resources"`
}

// https://docs.microsoft.com/en-us/nuget/api/service-index#resource
type ServiceResource struct {
	ID   string `json:"@id"`
	Type string `json:"@type"`
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#response
type RegistrationIndexResponse struct {
	RegistrationIndexURL string                   `json:"@id"`
	Type                 []string                 `json:"@type"`
	Count                int                      `json:"count"`
	Pages                []*RegistrationIndexPage `json:"items"`
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-page-object
type RegistrationIndexPage struct {
	RegistrationPageURL string                       `json:"@id"`
	Lower               string                       `json:"lower"`
	Upper               string                       `json:"upper"`
	Count               int                          `json:"count"`
	Items               []*RegistrationIndexPageItem `json:"items"`
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf-object-in-a-page
type RegistrationIndexPageItem struct {
	RegistrationLeafURL string        `json:"@id"`
	PackageContentURL   string        `json:"packageContent"`
	CatalogEntry        *CatalogEntry `json:"catalogEntry"`
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#catalog-entry
type CatalogEntry struct {
	CatalogLeafURL           string                    `json:"@id"`
	PackageContentURL        string                    `json:"packageContent"`
	ID                       string                    `json:"id"`
	Version                  string                    `json:"version"`
	Description              string                    `json:"description"`
	ReleaseNotes             string                    `json:"releaseNotes"`
	Authors                  string                    `json:"authors"`
	RequireLicenseAcceptance bool                      `json:"requireLicenseAcceptance"`
	ProjectURL               string                    `json:"projectURL"`
	DependencyGroups         []*PackageDependencyGroup `json:"dependencyGroups"`
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#package-dependency-group
type PackageDependencyGroup struct {
	TargetFramework string               `json:"targetFramework"`
	Dependencies    []*PackageDependency `json:"dependencies"`
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#package-dependency
type PackageDependency struct {
	ID    string `json:"id"`
	Range string `json:"range"`
}

func createRegistrationIndexResponse(l *linkBuilder, pds []*packages_model.PackageDescriptor) *RegistrationIndexResponse {
	sort.Slice(pds, func(i, j int) bool {
		return pds[i].SemVer.LessThan(pds[j].SemVer)
	})

	items := make([]*RegistrationIndexPageItem, 0, len(pds))
	for _, p := range pds {
		items = append(items, createRegistrationIndexPageItem(l, p))
	}

	return &RegistrationIndexResponse{
		RegistrationIndexURL: l.GetRegistrationIndexURL(pds[0].Package.Name),
		Type:                 []string{"catalog:CatalogRoot", "PackageRegistration", "catalog:Permalink"},
		Count:                1,
		Pages: []*RegistrationIndexPage{
			{
				RegistrationPageURL: l.GetRegistrationIndexURL(pds[0].Package.Name),
				Count:               len(pds),
				Lower:               pds[0].Version.Version,
				Upper:               pds[len(pds)-1].Version.Version,
				Items:               items,
			},
		},
	}
}

func createRegistrationIndexPageItem(l *linkBuilder, pd *packages_model.PackageDescriptor) *RegistrationIndexPageItem {
	metadata := pd.Metadata.(*nuget_module.Metadata)

	return &RegistrationIndexPageItem{
		RegistrationLeafURL: l.GetRegistrationLeafURL(pd.Package.Name, pd.Version.Version),
		PackageContentURL:   l.GetPackageDownloadURL(pd.Package.Name, pd.Version.Version),
		CatalogEntry: &CatalogEntry{
			CatalogLeafURL:    l.GetRegistrationLeafURL(pd.Package.Name, pd.Version.Version),
			PackageContentURL: l.GetPackageDownloadURL(pd.Package.Name, pd.Version.Version),
			ID:                pd.Package.Name,
			Version:           pd.Version.Version,
			Description:       metadata.Description,
			ReleaseNotes:      metadata.ReleaseNotes,
			Authors:           metadata.Authors,
			ProjectURL:        metadata.ProjectURL,
			DependencyGroups:  createDependencyGroups(pd),
		},
	}
}

func createDependencyGroups(pd *packages_model.PackageDescriptor) []*PackageDependencyGroup {
	metadata := pd.Metadata.(*nuget_module.Metadata)

	dependencyGroups := make([]*PackageDependencyGroup, 0, len(metadata.Dependencies))
	for k, v := range metadata.Dependencies {
		dependencies := make([]*PackageDependency, 0, len(v))
		for _, dep := range v {
			dependencies = append(dependencies, &PackageDependency{
				ID:    dep.ID,
				Range: dep.Version,
			})
		}

		dependencyGroups = append(dependencyGroups, &PackageDependencyGroup{
			TargetFramework: k,
			Dependencies:    dependencies,
		})
	}
	return dependencyGroups
}

// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf
type RegistrationLeafResponse struct {
	RegistrationLeafURL  string    `json:"@id"`
	Type                 []string  `json:"@type"`
	Listed               bool      `json:"listed"`
	PackageContentURL    string    `json:"packageContent"`
	Published            time.Time `json:"published"`
	RegistrationIndexURL string    `json:"registration"`
}

func createRegistrationLeafResponse(l *linkBuilder, pd *packages_model.PackageDescriptor) *RegistrationLeafResponse {
	return &RegistrationLeafResponse{
		Type:                 []string{"Package", "http://schema.nuget.org/catalog#Permalink"},
		Listed:               true,
		Published:            pd.Version.CreatedUnix.AsLocalTime(),
		RegistrationLeafURL:  l.GetRegistrationLeafURL(pd.Package.Name, pd.Version.Version),
		PackageContentURL:    l.GetPackageDownloadURL(pd.Package.Name, pd.Version.Version),
		RegistrationIndexURL: l.GetRegistrationIndexURL(pd.Package.Name),
	}
}

// https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#response
type PackageVersionsResponse struct {
	Versions []string `json:"versions"`
}

func createPackageVersionsResponse(pvs []*packages_model.PackageVersion) *PackageVersionsResponse {
	versions := make([]string, 0, len(pvs))
	for _, pv := range pvs {
		versions = append(versions, pv.Version)
	}

	return &PackageVersionsResponse{
		Versions: versions,
	}
}

// https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#response
type SearchResultResponse struct {
	TotalHits int64           `json:"totalHits"`
	Data      []*SearchResult `json:"data"`
}

// https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-result
type SearchResult struct {
	ID                   string                 `json:"id"`
	Version              string                 `json:"version"`
	Versions             []*SearchResultVersion `json:"versions"`
	Description          string                 `json:"description"`
	Authors              string                 `json:"authors"`
	ProjectURL           string                 `json:"projectURL"`
	RegistrationIndexURL string                 `json:"registration"`
}

// https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-result
type SearchResultVersion struct {
	RegistrationLeafURL string `json:"@id"`
	Version             string `json:"version"`
	Downloads           int64  `json:"downloads"`
}

func createSearchResultResponse(l *linkBuilder, totalHits int64, pds []*packages_model.PackageDescriptor) *SearchResultResponse {
	grouped := make(map[string][]*packages_model.PackageDescriptor)
	for _, pd := range pds {
		grouped[pd.Package.Name] = append(grouped[pd.Package.Name], pd)
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	collate.New(language.English, collate.IgnoreCase).SortStrings(keys)

	data := make([]*SearchResult, 0, len(pds))
	for _, key := range keys {
		data = append(data, createSearchResult(l, grouped[key]))
	}

	return &SearchResultResponse{
		TotalHits: totalHits,
		Data:      data,
	}
}

func createSearchResult(l *linkBuilder, pds []*packages_model.PackageDescriptor) *SearchResult {
	latest := pds[0]
	versions := make([]*SearchResultVersion, 0, len(pds))
	for _, pd := range pds {
		if latest.SemVer.LessThan(pd.SemVer) {
			latest = pd
		}

		versions = append(versions, &SearchResultVersion{
			RegistrationLeafURL: l.GetRegistrationLeafURL(pd.Package.Name, pd.Version.Version),
			Version:             pd.Version.Version,
		})
	}

	metadata := latest.Metadata.(*nuget_module.Metadata)

	return &SearchResult{
		ID:                   latest.Package.Name,
		Version:              latest.Version.Version,
		Versions:             versions,
		Description:          metadata.Description,
		Authors:              metadata.Authors,
		ProjectURL:           metadata.ProjectURL,
		RegistrationIndexURL: l.GetRegistrationIndexURL(latest.Package.Name),
	}
}
