// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
	packages_service "code.gitea.io/gitea/services/packages"
)

func RemovePackageVersion(ctx context.Context, doer *user_model.User, pv *packages_model.PackageVersion) error {
	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		return err
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
			return err
		}
		if err := BuildAllRepositoryFiles(ctx, pd.Owner.ID); err != nil {
			return fmt.Errorf("alpine.BuildAllRepositoryFiles failed: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	notify_service.PackageDelete(ctx, doer, pd)
	return nil
}
