// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"sync"

	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
)

type nop struct{}

func (n *nop) GetViewPackageVersionData(ctx context.Context, pd *packages_model.PackageDescriptor) (any, error) {
	return nil, nil //nolint:nilnil // no data, no error
}

func (n *nop) OnBeforeRemovePackageAll(ctx context.Context, doer *user_model.User, pkg *packages_model.Package, pds []*packages_model.PackageDescriptor) error {
	return nil
}

func (n *nop) OnBeforeRemovePackageVersion(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) error {
	return nil
}

var _ Specialization = (*nop)(nil)

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
		return &nop{}
	}
	return spec
}

var GetSpecManager = sync.OnceValue(func() *SpecManagerType {
	return &SpecManagerType{specMap: make(map[packages_model.Type]Specialization)}
})
