// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pypi

import (
	"code.gitea.io/gitea/models"
	pypi_module "code.gitea.io/gitea/modules/packages/pypi"

	jsoniter "github.com/json-iterator/go"
)

// Package represents a package with NPM metadata
type Package struct {
	*models.Package
	Files    []*models.PackageFile
	Metadata *pypi_module.Metadata
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
	var m *pypi_module.Metadata
	err := jsoniter.Unmarshal([]byte(p.MetadataRaw), &m)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &pypi_module.Metadata{}
	}

	pfs, err := p.GetFiles()
	if err != nil {
		return nil, err
	}

	return &Package{
		Package:  p,
		Files:    pfs,
		Metadata: m,
	}, nil
}
