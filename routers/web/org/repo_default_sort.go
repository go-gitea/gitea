// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/context"
)

func getOrgRepoDefaultSort(ctx *context.Context, org *organization.Organization) (string, error) {
	defaultSort, err := user_model.GetUserSetting(ctx, org.ID, user_model.SettingsKeyOrgRepoDefaultSort)
	if err != nil {
		return "", err
	}
	if defaultSort != "" {
		if _, ok := repo_model.OrderByFlatMap[defaultSort]; !ok {
			return "", nil
		}
	}
	return defaultSort, nil
}
