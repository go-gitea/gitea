// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cargo

import (
	packages_model "gitea.dev/models/packages"
	cargo_module "gitea.dev/modules/packages/cargo"
	api "gitea.dev/modules/structs"
	packages_service "gitea.dev/services/packages"
)

type Specialization struct {
	packages_service.SpecGetViewPackageVersionData
	packages_service.SpecOnBeforeRemovePackageAll
	packages_service.SpecOnBeforeRemovePackageVersion
	packages_service.SpecRenderUsageManual
	packages_service.SpecPublicPropertyNames
}

var (
	_ packages_service.Specialization          = (*Specialization)(nil)
	_ packages_service.PublicMetadataPublisher = (*Specialization)(nil)
)

// PublicMetadata publishes the dependencies, features, authors, license and so
// on of a version, which a client needs to publish the crate to another registry.
func (*Specialization) PublicMetadata(pd *packages_model.PackageDescriptor) *api.PackageMetadata {
	m, ok := pd.Metadata.(*cargo_module.Metadata)
	if !ok || m == nil {
		return nil
	}
	deps := make([]*api.PackageCargoDependency, 0, len(m.Dependencies))
	for _, d := range m.Dependencies {
		deps = append(deps, &api.PackageCargoDependency{
			Name:            d.Name,
			Req:             d.Req,
			Features:        d.Features,
			Optional:        d.Optional,
			DefaultFeatures: d.DefaultFeatures,
			Target:          d.Target,
			Kind:            d.Kind,
			Registry:        d.Registry,
			Package:         d.Package,
		})
	}
	return &api.PackageMetadata{Cargo: &api.PackageCargoMetadata{
		Dependencies:     deps,
		Features:         m.Features,
		Authors:          m.Authors,
		Description:      m.Description,
		DocumentationURL: m.DocumentationURL,
		ProjectURL:       m.ProjectURL,
		Readme:           m.Readme,
		Keywords:         m.Keywords,
		Categories:       m.Categories,
		License:          m.License,
		RepositoryURL:    m.RepositoryURL,
		Links:            m.Links,
	}}
}
