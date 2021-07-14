// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package npm

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"

	npm_module "code.gitea.io/gitea/modules/packages/npm"
)

func createPackageMetadataResponse(registryURL string, packages []*Package) *npm_module.PackageMetadata {
	sortedPackages := sortPackagesByVersionASC(packages)

	versions := make(map[string]*npm_module.PackageMetadataVersion)
	for _, p := range sortedPackages {
		versions[p.SemVer.String()] = createPackageMetadataVersion(registryURL, p)
	}

	latest := sortedPackages[len(sortedPackages)-1]

	distTags := make(map[string]string)
	distTags["latest"] = latest.Version

	return &npm_module.PackageMetadata{
		ID:          latest.Package.Name,
		Name:        latest.Package.Name,
		DistTags:    distTags,
		Description: latest.Metadata.Description,
		Readme:      latest.Metadata.Readme,
		Homepage:    latest.Metadata.Homepage,
		Author:      npm_module.User{Name: latest.Metadata.Author},
		License:     latest.Metadata.License,
		Versions:    versions,
	}
}

func createPackageMetadataVersion(registryURL string, p *Package) *npm_module.PackageMetadataVersion {
	hashBytes, _ := hex.DecodeString(p.PackageFile.HashSHA512)

	return &npm_module.PackageMetadataVersion{
		ID:           fmt.Sprintf("%s@%s", p.Package.Name, p.Package.Version),
		Name:         p.Package.Name,
		Version:      p.Package.Version,
		Description:  p.Metadata.Description,
		Author:       npm_module.User{Name: p.Metadata.Author},
		Homepage:     p.Metadata.Homepage,
		License:      p.Metadata.License,
		Dependencies: p.Metadata.Dependencies,
		Readme:       p.Metadata.Readme,
		Dist: npm_module.PackageDistribution{
			Shasum:    p.PackageFile.HashSHA1,
			Integrity: "sha512-" + base64.StdEncoding.EncodeToString(hashBytes),
			Tarball:   fmt.Sprintf("%s/%s/-/%s/%s", registryURL, url.QueryEscape(p.Package.Name), p.Package.Version, p.PackageFile.LowerName),
		},
	}
}
