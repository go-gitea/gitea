// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pypi

import (
	packages_model "gitea.dev/models/packages"
	pypi_module "gitea.dev/modules/packages/pypi"
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

// PublicMetadata publishes the author, summary, description, license and so
// on of a version, which a client needs to upload the file to another registry.
func (*Specialization) PublicMetadata(pd *packages_model.PackageDescriptor) *api.PackageMetadata {
	m, ok := pd.Metadata.(*pypi_module.Metadata)
	if !ok || m == nil {
		return nil
	}
	return &api.PackageMetadata{PyPI: &api.PackagePyPIMetadata{
		Author:          m.Author,
		Description:     m.Description,
		LongDescription: m.LongDescription,
		Summary:         m.Summary,
		ProjectURL:      m.ProjectURL,
		License:         m.License,
		RequiresPython:  m.RequiresPython,
	}}
}
