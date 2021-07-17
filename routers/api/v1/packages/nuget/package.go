// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"sort"

	"code.gitea.io/gitea/models"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"

	"github.com/hashicorp/go-version"
	jsoniter "github.com/json-iterator/go"
)

// Package represents a package with NuGet metadata
type Package struct {
	*models.Package
	SemVer   *version.Version
	Metadata *nuget_module.Metadata
}

func intializePackages(packages []*models.Package) ([]*Package, error) {
	pgs := make([]*Package, 0, len(packages))
	for _, p := range packages {
		np, err := intializePackage(p)
		if err != nil {
			return nil, err
		}
		pgs = append(pgs, np)
	}
	return pgs, nil
}

func intializePackage(p *models.Package) (*Package, error) {
	v, err := version.NewSemver(p.Version)
	if err != nil {
		return nil, err
	}

	var m *nuget_module.Metadata
	err = jsoniter.Unmarshal([]byte(p.MetadataRaw), &m)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &nuget_module.Metadata{}
	}
	return &Package{
		Package:  p,
		SemVer:   v,
		Metadata: m,
	}, nil
}

func sortPackagesByVersionASC(packages []*Package) []*Package {
	sortedPackages := make([]*Package, len(packages))
	copy(sortedPackages, packages)

	sort.Slice(sortedPackages, func(i, j int) bool {
		return sortedPackages[i].SemVer.LessThan(sortedPackages[j].SemVer)
	})

	return sortedPackages
}
