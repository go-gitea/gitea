// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// CanUserDelete returns true if user could delete the repository
func CanUserDelete(repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if user.IsAdmin || user.ID == repo.OwnerID {
		return true, nil
	}

	if err := repo.GetOwner(db.DefaultContext); err != nil {
		return false, err
	}

	if repo.Owner.IsOrganization() {
		isOwner, err := organization.OrgFromUser(repo.Owner).IsOwnedBy(user.ID)
		if err != nil {
			return false, err
		}
		return isOwner, nil
	}

	return false, nil
}
