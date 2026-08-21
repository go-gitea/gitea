// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"html/template"
	"sync"

	packages_model "gitea.dev/models/packages"
	user_model "gitea.dev/models/user"
)

type SpecGetViewPackageVersionData struct{}

func (*SpecGetViewPackageVersionData) GetViewPackageVersionData(ctx context.Context, pd *packages_model.PackageDescriptor) (any, error) {
	return nil, nil //nolint:nilnil // no data, no error
}

type SpecOnBeforeRemovePackageAll struct{}

func (*SpecOnBeforeRemovePackageAll) OnBeforeRemovePackageAll(ctx context.Context, doer *user_model.User, pkg *packages_model.Package, pds []*packages_model.PackageDescriptor) error {
	return nil
}

type SpecOnBeforeRemovePackageVersion struct{}

func (*SpecOnBeforeRemovePackageVersion) OnBeforeRemovePackageVersion(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) error {
	return nil
}

type SpecRenderUsageManual struct{}

func (*SpecRenderUsageManual) RenderSetupManual(ctx context.Context, pkg *packages_model.PackageDescriptor, viewData any) template.HTML {
	return ""
}

type specDefault struct {
	SpecGetViewPackageVersionData
	SpecOnBeforeRemovePackageAll
	SpecOnBeforeRemovePackageVersion
	SpecRenderUsageManual
}

func (n *specDefault) OnBeforeRemovePackageVersion(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) error {
	return nil
}

var _ Specialization = (*specDefault)(nil)

type SpecManagerType struct {
	specMap map[packages_model.Type]Specialization
}

func (m *SpecManagerType) Add(t packages_model.Type, spec Specialization) {
	m.specMap[t] = spec
}

func (m *SpecManagerType) Get(t packages_model.Type) Specialization {
	if len(m.specMap) == 0 {
		panic("specialization not initialized")
	}
	spec := m.specMap[t]
	if spec == nil {
		return &specDefault{}
	}
	return spec
}

var GetSpecManager = sync.OnceValue(func() *SpecManagerType {
	return &SpecManagerType{specMap: make(map[packages_model.Type]Specialization)}
})
