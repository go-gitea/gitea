// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	packages_model "gitea.dev/models/packages"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/json"
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
	RawMetadata       map[string]any `json:"-"`
	Name              string         `json:"name"`
	Version           string         `json:"version"`
	VersionNormalized string         `json:"version_normalized,omitempty"`
	Type              string         `json:"type"`
	Created           time.Time      `json:"time"`
	PublishedTime     time.Time      `json:"published-time,omitempty"`
	DefaultBranch     bool           `json:"default-branch,omitempty"`
	Dist              Dist           `json:"dist"`
	Source            Source         `json:"source"`
}

func (p PackageVersionMetadata) MarshalJSON() ([]byte, error) {
	type packageVersionMetadata PackageVersionMetadata
	data := make(map[string]any, len(p.RawMetadata)+8)
	for key, value := range p.RawMetadata {
		if key != "name" && key != "version" && key != "type" {
			data[key] = value
		}
	}
	bytes, err := json.Marshal((*packageVersionMetadata)(&p))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

// Dist contains package download information
type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Checksum  string `json:"shasum"`
	Reference string `json:"reference,omitempty"`
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
	commit, rawMetadata := loadDevBranchComposerMetadata(ctx, repo, branch)
	reference := branch
	created := pd.Version.CreatedUnix.AsLocalTime()
	if commit != nil {
		reference = commit.ID.String()
		created = commit.Committer.When
	}
	if rawPackageType, ok := rawMetadata["type"].(string); ok && rawPackageType != "" {
		packageType = rawPackageType
	}

	return &PackageVersionMetadata{
		Name:              pd.Package.Name,
		Version:           pd.Version.Version,
		VersionNormalized: pd.Version.Version,
		Type:              packageType,
		Created:           created,
		PublishedTime:     pd.Version.CreatedUnix.AsLocalTime(),
		DefaultBranch:     branch == repo.DefaultBranch,
		RawMetadata:       rawMetadata,
		Metadata:          pd.Metadata.(*composer_module.Metadata),
		Dist: Dist{
			Type:      "zip",
			URL:       repo.HTMLURL(ctx) + "/archive/" + util.PathEscapeSegments(branch) + ".zip",
			Reference: reference,
		},
		Source: Source{
			URL:       repo.HTMLURL(ctx),
			Type:      "git",
			Reference: reference,
		},
	}, true
}

func loadDevBranchComposerMetadata(ctx *context.Context, repo *repo_model.Repository, branch string) (*git.Commit, map[string]any) {
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		log.Error("OpenRepository[%s]: %v", repo.FullName(), err)
		return nil, nil
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit(branch)
	if err != nil {
		log.Error("GetBranchCommit[%s:%s]: %v", repo.FullName(), branch, err)
		return nil, nil
	}

	content, err := commit.GetFileContent("composer.json", 10*1024*1024)
	if err != nil {
		if !git.IsErrNotExist(err) && !errors.Is(err, git.ErrNotExist{}) {
			log.Error("GetFileContent[%s:%s:composer.json]: %v", repo.FullName(), branch, err)
		}
		return commit, nil
	}

	var metadata map[string]any
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		log.Error("Unmarshal composer.json metadata[%s:%s]: %v", repo.FullName(), branch, err)
		return commit, nil
	}

	return commit, metadata
}
