// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/agit"
	container_service "code.gitea.io/gitea/services/packages/container"
)

func renameUser(ctx context.Context, u *user_model.User, newUserName string) error {
	if u.IsOrganization() {
		return fmt.Errorf("cannot rename organization")
	}

	if err := user_model.ChangeUserName(ctx, u, newUserName); err != nil {
		return err
	}

	if err := agit.UserNameChanged(ctx, u, newUserName); err != nil {
		return err
	}
	if err := container_service.UpdateRepositoryNames(ctx, u, newUserName); err != nil {
		return err
	}

	u.Name = newUserName
	u.LowerName = strings.ToLower(newUserName)
	if err := user_model.UpdateUser(ctx, u, false); err != nil {
		return err
	}

	log.Trace("User name changed: %s -> %s", u.Name, newUserName)
	return nil
}
