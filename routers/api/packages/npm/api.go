// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"

	packages_model "code.gitea.io/gitea/models/packages"
	npm_module "code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"
)

func createPackageMetadataResponse(registryURL string, pds []*packages_model.PackageDescriptor) *npm_module.PackageMetadata {
	sort.Slice(pds, func(i, j int) bool {
		return pds[i].SemVer.LessThan(pds[j].SemVer)
	})

	versions := make(map[string]*npm_module.PackageMetadataVersion)
	distTags := make(map[string]string)
	for _, pd := range pds {
		versions[pd.SemVer.String()] = createPackageMetadataVersion(registryURL, pd)

		for _, pvp := range pd.VersionProperties {
			if pvp.Name == npm_module.TagProperty {
				distTags[pvp.Value] = pd.Version.Version
			}
		}
	}

	latest := pds[len(pds)-1]

	metadata := latest.Metadata.(*npm_module.Metadata)

	return &npm_module.PackageMetadata{
		ID:          latest.Package.Name,
		Name:        latest.Package.Name,
		DistTags:    distTags,
		Description: metadata.Description,
		Readme:      metadata.Readme,
		Homepage:    metadata.ProjectURL,
		Author:      npm_module.User{Name: metadata.Author},
		License:     metadata.License,
		Versions:    versions,
		Repository:  metadata.Repository,
	}
}

func createPackageMetadataVersion(registryURL string, pd *packages_model.PackageDescriptor) *npm_module.PackageMetadataVersion {
	hashBytes, _ := hex.DecodeString(pd.Files[0].Blob.HashSHA512)

	metadata := pd.Metadata.(*npm_module.Metadata)

	return &npm_module.PackageMetadataVersion{
		ID:                   fmt.Sprintf("%s@%s", pd.Package.Name, pd.Version.Version),
		Name:                 pd.Package.Name,
		Version:              pd.Version.Version,
		Description:          metadata.Description,
		Author:               npm_module.User{Name: metadata.Author},
		Homepage:             metadata.ProjectURL,
		License:              metadata.License,
		Dependencies:         metadata.Dependencies,
		BundleDependencies:   metadata.BundleDependencies,
		DevDependencies:      metadata.DevelopmentDependencies,
		PeerDependencies:     metadata.PeerDependencies,
		OptionalDependencies: metadata.OptionalDependencies,
		Readme:               metadata.Readme,
		Bin:                  metadata.Bin,
		Dist: npm_module.PackageDistribution{
			Shasum:    pd.Files[0].Blob.HashSHA1,
			Integrity: "sha512-" + base64.StdEncoding.EncodeToString(hashBytes),
			Tarball:   fmt.Sprintf("%s/%s/-/%s/%s", registryURL, url.QueryEscape(pd.Package.Name), url.PathEscape(pd.Version.Version), url.PathEscape(pd.Files[0].File.LowerName)),
		},
	}
}

func createPackageSearchResponse(pds []*packages_model.PackageDescriptor, total int64) *npm_module.PackageSearch {
	objects := make([]*npm_module.PackageSearchObject, 0, len(pds))
	for _, pd := range pds {
		metadata := pd.Metadata.(*npm_module.Metadata)

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
