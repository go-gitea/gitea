// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"context"

	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
)

type Specialization struct{}

var _ packages_service.Specialization = (*Specialization)(nil)

func (s Specialization) OnBeforeRemovePackageAll(ctx context.Context, doer *user_model.User, pkg *packages_model.Package, pds []*packages_model.PackageDescriptor) error {
	locked, err := IsLocked(ctx, pkg)
	if err != nil {
		return err
	}
	if locked {
		return util.ErrorWrapTranslatable(
			util.ErrUnprocessableContent,
			"packages.terraform.delete.locked",
		)
	}
	return nil
}

func (s Specialization) OnBeforeRemovePackageVersion(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) error {
	locked, err := IsLocked(ctx, pd.Package)
	if err != nil {
		return err
	}
	if locked {
		return util.ErrorWrapTranslatable(
			util.ErrUnprocessableContent,
			"packages.terraform.delete.locked",
		)
	}

	latest, err := IsLatest(ctx, pd)
	if err != nil {
		return err
	}
	if latest {
		return util.ErrorWrapTranslatable(
			util.ErrUnprocessableContent,
			"packages.terraform.delete.latest",
		)
	}
	return nil
}
