// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"time"

	packages_model "gitea.dev/models/packages"
	npm_module "gitea.dev/modules/packages/npm"
	"gitea.dev/modules/setting"
)

func createPackageMetadataResponse(registryURL string, pds []*packages_model.PackageDescriptor) *npm_module.PackageMetadata {
	sort.Slice(pds, func(i, j int) bool {
		return pds[i].SemVer.LessThan(pds[j].SemVer)
	})

	versions := make(map[string]*npm_module.PackageMetadataVersion)
	distTags := make(map[string]string)
	times := make(map[string]time.Time)
	firstPublished, lastPublished := pds[0].Version.CreatedUnix, pds[0].Version.CreatedUnix
	for _, pd := range pds {
		semVer := pd.SemVer.String()
		versions[semVer] = createPackageMetadataVersion(registryURL, pd)
		times[semVer] = pd.Version.CreatedUnix.AsTimeInLocation(time.UTC)
		firstPublished = min(firstPublished, pd.Version.CreatedUnix)
		lastPublished = max(lastPublished, pd.Version.CreatedUnix)

		for _, pvp := range pd.VersionProperties {
			if pvp.Name == npm_module.TagProperty {
				distTags[pvp.Value] = pd.Version.Version
			}
		}
	}

	// npm derives both from the versions currently served, so a deletion moves them
	times["created"] = firstPublished.AsTimeInLocation(time.UTC)
	times["modified"] = lastPublished.AsTimeInLocation(time.UTC)

	latest := pds[len(pds)-1]

	metadata := packages_model.DescriptorMetadata[*npm_module.Metadata](latest)

	return &npm_module.PackageMetadata{
		ID:          latest.Package.Name,
		Name:        latest.Package.Name,
		DistTags:    distTags,
		Description: metadata.Description,
		Readme:      metadata.Readme,
		Maintainers: []npm_module.User{{Name: latest.Owner.Name}},
		Time:        times,
		Homepage:    metadata.ProjectURL,
		Keywords:    metadata.Keywords,
		Author:      npm_module.User{Name: metadata.Author},
		License:     metadata.License,
		Versions:    versions,
		Repository:  metadata.Repository,
	}
}

func createPackageMetadataVersion(registryURL string, pd *packages_model.PackageDescriptor) *npm_module.PackageMetadataVersion {
	hashBytes, _ := hex.DecodeString(pd.Files[0].Blob.HashSHA512)

	metadata := packages_model.DescriptorMetadata[*npm_module.Metadata](pd)

	return &npm_module.PackageMetadataVersion{
		ID:                   fmt.Sprintf("%s@%s", pd.Package.Name, pd.Version.Version),
		Name:                 pd.Package.Name,
		Version:              pd.Version.Version,
		Description:          metadata.Description,
		Author:               npm_module.User{Name: metadata.Author},
		Maintainers:          []npm_module.User{{Name: pd.Owner.Name}},
		Homepage:             metadata.ProjectURL,
		License:              metadata.License,
		Keywords:             metadata.Keywords,
		Dependencies:         metadata.Dependencies,
		BundleDependencies:   metadata.BundleDependencies,
		DevDependencies:      metadata.DevelopmentDependencies,
		PeerDependencies:     metadata.PeerDependencies,
		PeerDependenciesMeta: metadata.PeerDependenciesMeta,
		OptionalDependencies: metadata.OptionalDependencies,
		Readme:               metadata.Readme,
		Bin:                  metadata.Bin,
		HasInstallScript:     metadata.HasInstallScript,
		HasShrinkwrap:        metadata.HasShrinkwrap,
		Engines:              metadata.Engines,
		CPU:                  metadata.CPU,
		OS:                   metadata.OS,
		Directories:          metadata.Directories,
		Funding:              metadata.Funding,
		AcceptDependencies:   metadata.AcceptDependencies,
		Deprecated:           metadata.Deprecated,
		Dist: npm_module.PackageDistribution{
			Shasum:    pd.Files[0].Blob.HashSHA1,
			Integrity: "sha512-" + base64.StdEncoding.EncodeToString(hashBytes),
			Tarball:   fmt.Sprintf("%s/%s/-/%s/%s", registryURL, url.PathEscape(pd.Package.Name), url.PathEscape(pd.Version.Version), url.PathEscape(pd.Files[0].File.LowerName)),
		},
	}
}

func createPackageSearchResponse(pds []*packages_model.PackageDescriptor, total int64) *npm_module.PackageSearch {
	objects := make([]*npm_module.PackageSearchObject, 0, len(pds))
	for _, pd := range pds {
		metadata := packages_model.DescriptorMetadata[*npm_module.Metadata](pd)

		scope := metadata.Scope
		if scope == "" {
			scope = "unscoped"
		}

		objects = append(objects, &npm_module.PackageSearchObject{
			Package: &npm_module.PackageSearchPackage{
				Scope:       scope,
				Name:        metadata.Name,
				Version:     pd.Version.Version,
				Date:        pd.Version.CreatedUnix.AsLocalTime(),
				Description: metadata.Description,
				Author:      npm_module.User{Name: metadata.Author},
				Publisher:   npm_module.User{Name: pd.Owner.Name},
				Maintainers: []npm_module.User{}, // npm cli needs this field
				Keywords:    metadata.Keywords,
				Links: &npm_module.PackageSearchPackageLinks{
					Registry: setting.AppURL + "api/packages/" + pd.Owner.Name + "/npm",
					Homepage: metadata.ProjectURL,
				},
			},
		})
	}

	return &npm_module.PackageSearch{
		Objects: objects,
		Total:   total,
	}
}
