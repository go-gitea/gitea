// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"bytes"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
)

// ServiceIndexResponse https://docs.microsoft.com/en-us/nuget/api/service-index#resources
type ServiceIndexResponse struct {
	Version   string     `json:"version"`
	Resources []ServiceResource `json:"resources"`
}

// ServiceResource https://docs.microsoft.com/en-us/nuget/api/service-index#resource
type ServiceResource struct {
	ID   string `json:"@id"`
	Type string `json:"@type"`
}

func createServiceIndexResponse(root string) *ServiceIndexResponse {
	return &ServiceIndexResponse{
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
		},
	}
}

// RegistrationIndexResponse https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#response
type RegistrationIndexResponse struct {
	RegistrationIndexURL string                   `json:"@id"`
	Type                 []string                 `json:"@type"`
	Count                int                      `json:"count"`
	Pages                []*RegistrationIndexPage `json:"items"`
}

// RegistrationIndexPage https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-page-object
type RegistrationIndexPage struct {
	RegistrationPageURL string                       `json:"@id"`
	Lower               string                       `json:"lower"`
	Upper               string                       `json:"upper"`
	Count               int                          `json:"count"`
	Items               []*RegistrationIndexPageItem `json:"items"`
}

// RegistrationIndexPageItem https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf-object-in-a-page
type RegistrationIndexPageItem struct {
	RegistrationLeafURL string        `json:"@id"`
	PackageContentURL   string        `json:"packageContent"`
	CatalogEntry        *CatalogEntry `json:"catalogEntry"`
}

// CatalogEntry https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#catalog-entry
type CatalogEntry struct {
	CatalogLeafURL           string                    `json:"@id"`
	PackageContentURL        string                    `json:"packageContent"`
	ID                       string                    `json:"id"`
	Version                  string                    `json:"version"`
	Description              string                    `json:"description"`
	Summary                  string                    `json:"summary"`
	ReleaseNotes             string                    `json:"releaseNotes"`
	Authors                  string                    `json:"authors"`
	RequireLicenseAcceptance bool                      `json:"requireLicenseAcceptance"`
	ProjectURL               string                    `json:"projectURL"`
	DependencyGroups         []*PackageDependencyGroup `json:"dependencyGroups"`
}

// PackageDependencyGroup https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#package-dependency-group
type PackageDependencyGroup struct {
	TargetFramework string               `json:"targetFramework"`
	Dependencies    []*PackageDependency `json:"dependencies"`
}

// PackageDependency https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#package-dependency
type PackageDependency struct {
	ID    string `json:"id"`
	Range string `json:"range"`
}

func createRegistrationIndexResponse(l *linkBuilder, packages []*Package) *RegistrationIndexResponse {
	sortedPackages := sortPackagesByVersionASC(packages)

	items := make([]*RegistrationIndexPageItem, 0, len(packages))
	for _, p := range sortedPackages {
		items = append(items, createRegistrationIndexPageItem(l, p))
	}

	return &RegistrationIndexResponse{
		RegistrationIndexURL: l.GetRegistrationIndexURL(packages[0].Name),
		Type:                 []string{"catalog:CatalogRoot", "PackageRegistration", "catalog:Permalink"},
		Count:                1,
		Pages: []*RegistrationIndexPage{
			{
				RegistrationPageURL: l.GetRegistrationIndexURL(packages[0].Name),
				Count:               len(packages),
				Lower:               normalizeVersion(packages[0].SemVer),
				Upper:               normalizeVersion(packages[len(packages)-1].SemVer),
				Items:               items,
			},
		},
	}
}

func createRegistrationIndexPageItem(l *linkBuilder, p *Package) *RegistrationIndexPageItem {
	return &RegistrationIndexPageItem{
		RegistrationLeafURL: l.GetRegistrationLeafURL(p.Name, p.Version),
		PackageContentURL:   l.GetPackageDownloadURL(p.Name, p.Version),
		CatalogEntry: &CatalogEntry{
			CatalogLeafURL:           l.GetRegistrationLeafURL(p.Name, p.Version),
			PackageContentURL:        l.GetPackageDownloadURL(p.Name, p.Version),
			ID:                       p.Name,
			Version:                  p.Version,
			Description:              p.Metadata.Description,
			Summary:                  p.Metadata.Summary,
			ReleaseNotes:             p.Metadata.ReleaseNotes,
			Authors:                  p.Metadata.Authors,
			RequireLicenseAcceptance: p.Metadata.RequireLicenseAcceptance,
			ProjectURL:               p.Metadata.ProjectURL,
			DependencyGroups:         createDependencyGroups(p),
		},
	}
}

func createDependencyGroups(p *Package) []*PackageDependencyGroup {
	dependencyGroups := make([]*PackageDependencyGroup, 0, len(p.Metadata.Dependencies))
	for k, v := range p.Metadata.Dependencies {
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

// RegistrationLeafResponse https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf
type RegistrationLeafResponse struct {
	RegistrationLeafURL  string    `json:"@id"`
	Type                 []string  `json:"@type"`
	Listed               bool      `json:"listed"`
	PackageContentURL    string    `json:"packageContent"`
	Published            time.Time `json:"published"`
	RegistrationIndexURL string    `json:"registration"`
}

func createRegistrationLeafResponse(l *linkBuilder, p *Package) *RegistrationLeafResponse {
	return &RegistrationLeafResponse{
		Type:                 []string{"Package", "http://schema.nuget.org/catalog#Permalink"},
		Listed:               true,
		Published:            time.Unix(int64(p.CreatedUnix), 0),
		RegistrationLeafURL:  l.GetRegistrationLeafURL(p.Name, p.Version),
		PackageContentURL:    l.GetPackageDownloadURL(p.Name, p.Version),
		RegistrationIndexURL: l.GetRegistrationIndexURL(p.Name),
	}
}

// PackageVersionsResponse https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#response
type PackageVersionsResponse struct {
	Versions []string `json:"versions"`
}

func createPackageVersionsResponse(packages []*Package) *PackageVersionsResponse {
	versions := make([]string, 0, len(packages))
	for _, p := range packages {
		versions = append(versions, normalizeVersion(p.SemVer))
	}

	return &PackageVersionsResponse{
		Versions: versions,
	}
}

// SearchResultResponse https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#response
type SearchResultResponse struct {
	TotalHits int64           `json:"totalHits"`
	Data      []*SearchResult `json:"data"`
}

// SearchResult https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-result
type SearchResult struct {
	ID                   string                 `json:"id"`
	Version              string                 `json:"version"`
	Versions             []*SearchResultVersion `json:"versions"`
	Description          string                 `json:"description"`
	Summary              string                 `json:"summary"`
	Authors              string                 `json:"authors"`
	ProjectURL           string                 `json:"projectURL"`
	RegistrationIndexURL string                 `json:"registration"`
}

// SearchResultVersion https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-result
type SearchResultVersion struct {
	RegistrationLeafURL string `json:"@id"`
	Version             string `json:"version"`
	Downloads           int64  `json:"downloads"`
}

func createSearchResultResponse(l *linkBuilder, totalHits int64, packages []*Package) *SearchResultResponse {
	data := make([]*SearchResult, 0, len(packages))

	if len(packages) > 0 {
		groupID := packages[0].Name
		group := make([]*Package, 0, 10)

		for i := 0; i < len(packages); i++ {
			if groupID != packages[i].Name {
				data = append(data, createSearchResult(l, group))
				groupID = packages[i].Name
				group = group[:0]
			}
			group = append(group, packages[i])
		}
		data = append(data, createSearchResult(l, group))
	}

	return &SearchResultResponse{
		TotalHits: totalHits,
		Data:      data,
	}
}

func createSearchResult(l *linkBuilder, packages []*Package) *SearchResult {
	latest := packages[0]
	versions := make([]*SearchResultVersion, 0, len(packages))
	for _, p := range packages {
		if latest.SemVer.LessThan(p.SemVer) {
			latest = p
		}

		versions = append(versions, &SearchResultVersion{
			RegistrationLeafURL: l.GetRegistrationLeafURL(p.Name, p.Version),
			Version:             p.Version,
		})
	}

	return &SearchResult{
		ID:                   latest.Name,
		Version:              latest.Version,
		Versions:             versions,
		Description:          latest.Metadata.Description,
		Summary:              latest.Metadata.Summary,
		Authors:              latest.Metadata.Authors,
		ProjectURL:           latest.Metadata.ProjectURL,
		RegistrationIndexURL: l.GetRegistrationIndexURL(latest.Name),
	}
}

// normalizeVersion removes the metadata
func normalizeVersion(v *version.Version) string {
	var buf bytes.Buffer
	segments := v.Segments64()
	fmt.Fprintf(&buf, "%d.%d.%d", segments[0], segments[1], segments[2])
	pre := v.Prerelease()
	if pre != "" {
		fmt.Fprintf(&buf, "-%s", pre)
	}
	return buf.String()
}
