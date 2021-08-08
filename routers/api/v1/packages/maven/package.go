// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package maven

import (
	"sort"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/json"
	maven_module "code.gitea.io/gitea/modules/packages/maven"
)

// Package represents a package with Maven metadata
type Package struct {
	*models.Package
	Metadata *maven_module.Metadata
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
	var m *maven_module.Metadata
	err := json.Unmarshal([]byte(p.MetadataRaw), &m)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &maven_module.Metadata{}
	}

	return &Package{
		Package:  p,
		Metadata: m,
	}, nil
}

func sortPackagesByVersionASC(packages []*Package) []*Package {
	sortedPackages := make([]*Package, len(packages))
	copy(sortedPackages, packages)

	sort.Slice(sortedPackages, func(i, j int) bool {
		return sortedPackages[i].Version < sortedPackages[j].Version
	})

	return sortedPackages
}
