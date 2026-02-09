// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"context"
	"errors"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

// DANGER delayed permission checks per package type in handler
func FineGrainedPackageWriteCheck(ctx context.Context, doer *user_model.User, ownerID int64, pkgType packages_model.Type, pkgName string, oldNames ...string) (ok bool, err error) {
	if doer.IsGiteaActions() {
		taskID, ok := user_model.GetActionsUserTaskID(doer)
		if !ok {
			return false, nil
		}
		task, err := actions_model.GetTaskByID(ctx, taskID)
		if err != nil {
			return false, err
		}

		pkg, err := packages_model.GetPackageByName(ctx, ownerID, pkgType, pkgName)
		if err != nil {
			// Allow package creation if package does not exist, create a linked package
			if errors.Is(err, util.ErrNotExist) {
				// maven pkg has an old name format, try to find it by old names before creating a new one
				for _, oldName := range oldNames {
					oldPkg, err := packages_model.GetPackageByName(ctx, ownerID, pkgType, oldName)
					if err == nil {
						pkg = oldPkg
						break
					}
					if !errors.Is(err, util.ErrNotExist) {
						return false, err
					}
				}

				p, err := packages_model.TryInsertPackage(ctx, &packages_model.Package{
					OwnerID:   ownerID,
					Name:      pkgName,
					LowerName: strings.ToLower(pkgName),
					Type:      pkgType,
					RepoID:    task.RepoID,
				})
				if err != nil {
					return false, err
				}
				pkg = p
			} else {
				return false, err
			}
		}
		if pkg.RepoID != task.RepoID {
			return false, nil
		}
	}
	return true, nil
}
