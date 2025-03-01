// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"fmt"
	"net/url"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	composer_module "code.gitea.io/gitea/modules/packages/composer"
)

// ServiceIndexResponse contains registry endpoints
type ServiceIndexResponse struct {
	SearchTemplate   string `json:"search"`
	MetadataTemplate string `json:"metadata-url"`
	PackageList      string `json:"list"`
}

func createServiceIndexResponse(registryURL string) *ServiceIndexResponse {
	return &ServiceIndexResponse{
		SearchTemplate:   registryURL + "/search.json?q=%query%&type=%type%",
		MetadataTemplate: registryURL + "/p2/%package%.json",
		PackageList:      registryURL + "/list.json",
	}
}

// SearchResultResponse contains search results
type SearchResultResponse struct {
	Total    int64           `json:"total"`
	Results  []*SearchResult `json:"results"`
	NextLink string          `json:"next,omitempty"`
}

// SearchResult contains a search result
type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Downloads   int64  `json:"downloads"`
}

func createSearchResultResponse(total int64, pds []*packages_model.PackageDescriptor, nextLink string) *SearchResultResponse {
	results := make([]*SearchResult, 0, len(pds))

	for _, pd := range pds {
		results = append(results, &SearchResult{
			Name:        pd.Package.Name,
			Description: pd.Metadata.(*composer_module.Metadata).Description,
			Downloads:   pd.Version.DownloadCount,
		})
	}

	return &SearchResultResponse{
		Total:    total,
		Results:  results,
		NextLink: nextLink,
	}
}

// PackageMetadataResponse contains packages metadata
type PackageMetadataResponse struct {
	Minified string                               `json:"minified"`
	Packages map[string][]*PackageVersionMetadata `json:"packages"`
}

// PackageVersionMetadata contains package metadata
// https://getcomposer.org/doc/05-repositories.md#package
type PackageVersionMetadata struct {
	*composer_module.Metadata
	Name    string    `json:"name"`
	Version string    `json:"version"`
	Type    string    `json:"type"`
	Created time.Time `json:"time"`
	Dist    Dist      `json:"dist"`
	Source  Source    `json:"source"`
}

// Dist contains package download information
type Dist struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Checksum string `json:"shasum"`
}

// Source contains package source information
type Source struct {
	URL       string `json:"url"`
	Type      string `json:"type"`
	Reference string `json:"reference"`
}

func createPackageMetadataResponse(registryURL string, pds []*packages_model.PackageDescriptor) *PackageMetadataResponse {
	versions := make([]*PackageVersionMetadata, 0, len(pds))

	for _, pd := range pds {
		packageType := ""
		for _, pvp := range pd.VersionProperties {
			if pvp.Name == composer_module.TypeProperty {
				packageType = pvp.Value
				break
			}
		}

		pkg := PackageVersionMetadata{
			Name:     pd.Package.Name,
			Version:  pd.Version.Version,
			Type:     packageType,
			Created:  pd.Version.CreatedUnix.AsLocalTime(),
			Metadata: pd.Metadata.(*composer_module.Metadata),
			Dist: Dist{
				Type:     "zip",
				URL:      fmt.Sprintf("%s/files/%s/%s/%s", registryURL, url.PathEscape(pd.Package.LowerName), url.PathEscape(pd.Version.LowerVersion), url.PathEscape(pd.Files[0].File.LowerName)),
				Checksum: pd.Files[0].Blob.HashSHA1,
			},
		}
		if pd.Repository != nil {
			pkg.Source = Source{
				URL:       pd.Repository.HTMLURL(),
				Type:      "git",
				Reference: pd.Version.Version,
			}
		}

		versions = append(versions, &pkg)
	}

	return &PackageMetadataResponse{
		Minified: "composer/2.0",
		Packages: map[string][]*PackageVersionMetadata{
			pds[0].Package.Name: versions,
		},
	}
}
