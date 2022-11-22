// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"context"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	container_module "code.gitea.io/gitea/modules/packages/container"
)

// UpdateRepositoryNames updates the repository name property for all packages of the specific owner
func UpdateRepositoryNames(ctx context.Context, owner *user_model.User, newOwnerName string) error {
	ps, err := packages_model.GetPackagesByType(ctx, owner.ID, packages_model.TypeContainer)
	if err != nil {
		return err
	}

	newOwnerName = strings.ToLower(newOwnerName)

	for _, p := range ps {
		if err := packages_model.DeletePropertyByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository); err != nil {
			return err
		}

		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, newOwnerName+"/"+p.LowerName); err != nil {
			return err
		}
	}

	return nil
}
