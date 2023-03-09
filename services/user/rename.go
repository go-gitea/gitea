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

func renameUser(ctx context.Context, u *user_model.User, newUserName string) (err error) {
	if u.IsOrganization() {
		return fmt.Errorf("cannot rename organization")
	}

	if u.LowerName == strings.ToLower(newUserName) {
		return fmt.Errorf("new username is the same as the old one")
	}

	if err := user_model.ChangeUserName(u, newUserName); err != nil {
		return err
	}

	err = agit.UserNameChanged(u, newUserName)
	if err != nil {
		return err
	}
	if err = container_service.UpdateRepositoryNames(ctx, u, newUserName); err != nil {
		return err
	}

	u.Name = newUserName
	u.LowerName = strings.ToLower(newUserName)
	if err := user_model.UpdateUser(ctx, u, false); err != nil {
		return err
	}

	log.Trace("User name changed: %s -> %s", u.Name, newUserName)
	return

}
