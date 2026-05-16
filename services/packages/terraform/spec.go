// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"context"

	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	terraform_module "code.gitea.io/gitea/modules/packages/terraform"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
)

type Specialization struct{}

var _ packages_service.Specialization = (*Specialization)(nil)

func (s Specialization) GetViewPackageVersionData(ctx context.Context, pd *packages_model.PackageDescriptor) (any, error) {
	var ret struct {
		IsLatestVersion bool
		TerraformLock   *terraform_module.LockInfo
	}
	latestPvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  pd.Package.ID,
		IsInternal: optional.Some(false),
	})
	if err != nil {
		return ret, err
	}
	isLatest := len(latestPvs) > 0 && latestPvs[0].ID == pd.Version.ID
	ret.IsLatestVersion = isLatest

	if isLatest {
		lockInfo, err := terraform_module.GetLock(ctx, pd.Package.ID)
		if err != nil {
			return ret, nil
		}
		if lockInfo.IsLocked() {
			ret.TerraformLock = &lockInfo
		}
	}
	return ret, nil
}

func (s Specialization) OnBeforeRemovePackageAll(ctx context.Context, doer *user_model.User, pkg *packages_model.Package, pds []*packages_model.PackageDescriptor) error {
	locked, err := IsLocked(ctx, pkg)
	if err != nil {
		return err
	}
	if locked {
		return util.ErrorWrapTranslatable(
			util.ErrorWrap(util.ErrUnprocessableContent, "terraform state is locked and cannot be deleted"),
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
			util.ErrorWrap(util.ErrUnprocessableContent, "terraform state is locked and cannot be deleted"),
			"packages.terraform.delete.locked",
		)
	}

	latest, err := IsLatest(ctx, pd)
	if err != nil {
		return err
	}
	if latest {
		return util.ErrorWrapTranslatable(
			util.ErrorWrap(util.ErrUnprocessableContent, "the latest version of a Terraform state cannot be deleted"),
			"packages.terraform.delete.latest",
		)
	}
	return nil
}
