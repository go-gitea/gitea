// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	packages_model "gitea.dev/models/packages"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	composer_module "gitea.dev/modules/packages/composer"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
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

func createPackageMetadataResponse(ctx *context.Context, registryURL string, pkg *packages_model.Package, pds []*packages_model.PackageDescriptor, includeDev bool) *PackageMetadataResponse {
	versions := make([]*PackageVersionMetadata, 0, len(pds))

	for _, pd := range pds {
		packageType := ""
		for _, pvp := range pd.VersionProperties {
			if pvp.Name == composer_module.TypeProperty {
				packageType = pvp.Value
				break
			}
		}
		isDevBranch := pd.VersionProperties.GetByName(composer_module.DevBranchProperty) != ""
		if isDevBranch != includeDev {
			continue
		}
		if isDevBranch {
			branchVersion, ok := createDevBranchPackageMetadata(ctx, packageType, pd)
			if ok {
				versions = append(versions, branchVersion)
			}
			continue
		}
		if len(pd.Files) == 0 {
			log.Error("Composer package version without files: %d", pd.Version.ID)
			continue
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
			permission, err := access_model.GetDoerRepoPermission(ctx, pd.Repository, ctx.Doer)
			if err != nil {
				log.Error("GetDoerRepoPermission[%d]: %v", pd.Repository.ID, err)
			} else if permission.HasAnyUnitAccessOrPublicAccess() {
				pkg.Source = Source{
					URL:       pd.Repository.HTMLURL(),
					Type:      "git",
					Reference: pd.Version.Version,
				}
			}
		}

		versions = append(versions, &pkg)
	}

	return &PackageMetadataResponse{
		Minified: "composer/2.0",
		Packages: map[string][]*PackageVersionMetadata{
			pkg.Name: versions,
		},
	}
}

func createDevBranchPackageMetadata(ctx *context.Context, packageType string, pd *packages_model.PackageDescriptor) (*PackageVersionMetadata, bool) {
	repoID := pd.Package.RepoID
	repoIDValue := pd.VersionProperties.GetByName(composer_module.DevBranchRepoProperty)
	if repoIDValue != "" {
		parsedRepoID, err := strconv.ParseInt(repoIDValue, 10, 64)
		if err != nil || parsedRepoID <= 0 {
			log.Error("Invalid Composer dev branch repository ID for package version %d", pd.Version.ID)
			return nil, false
		}
		repoID = parsedRepoID
	}
	if repoID <= 0 {
		log.Error("Missing Composer dev branch repository ID for package version %d", pd.Version.ID)
		return nil, false
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		log.Error("GetRepositoryByID[%d]: %v", repoID, err)
		return nil, false
	}
	permission, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		log.Error("GetDoerRepoPermission[%d]: %v", repo.ID, err)
		return nil, false
	}
	if !permission.HasAnyUnitAccessOrPublicAccess() {
		return nil, false
	}

	branch := pd.VersionProperties.GetByName(composer_module.DevBranchProperty)
	if branch == "" {
		branch = pd.Version.Version[len("dev-"):]
	}

	return &PackageVersionMetadata{
		Name:     pd.Package.Name,
		Version:  pd.Version.Version,
		Type:     packageType,
		Created:  pd.Version.CreatedUnix.AsLocalTime(),
		Metadata: pd.Metadata.(*composer_module.Metadata),
		Dist: Dist{
			Type: "zip",
			URL:  repo.HTMLURL(ctx) + "/archive/" + util.PathEscapeSegments(branch) + ".zip",
		},
		Source: Source{
			URL:       repo.HTMLURL(ctx),
			Type:      "git",
			Reference: branch,
		},
	}, true
}
