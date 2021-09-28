// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rubygems

import (
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	rubygems_module "code.gitea.io/gitea/modules/packages/rubygems"
)

// Package represents a package with RubyGems metadata
type Package struct {
	*packages.Package
	Metadata *rubygems_module.Metadata
}

func intializePackages(packages []*packages.Package) ([]*Package, error) {
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

func intializePackage(p *packages.Package) (*Package, error) {
	var m *rubygems_module.Metadata
	if err := json.Unmarshal([]byte(p.MetadataRaw), &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = &rubygems_module.Metadata{}
	}

	return &Package{
		Package:  p,
		Metadata: m,
	}, nil
}

// AsSpecification creates a Ruby Gem::Specification object used by ServePackageSpecification
func (p *Package) AsSpecification() *rubygems_module.RubyUserDef {
	return &rubygems_module.RubyUserDef{
		Name: "Gem::Specification",
		Value: []interface{}{
			"3.2.3", // @rubygems_version
			4,       // @specification_version,
			p.Name,
			&rubygems_module.RubyUserMarshal{
				Name:  "Gem::Version",
				Value: []string{p.Version},
			},
			nil,                 // date
			p.Metadata.Summary,  // @summary
			nil,                 // @required_ruby_version
			nil,                 // @required_rubygems_version
			p.Metadata.Platform, // @original_platform
			[]interface{}{},     // @dependencies
			nil,                 // rubyforge_project
			"",                  // @email
			p.Metadata.Authors,
			p.Metadata.Description,
			p.Metadata.ProjectURL,
			true,                // has_rdoc
			p.Metadata.Platform, // @new_platform
			nil,
			p.Metadata.Licenses,
		},
	}
}
