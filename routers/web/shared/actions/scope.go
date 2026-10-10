// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"

	"gitea.dev/modules/setting"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

// settingsScope is the repo, owner or instance an actions settings page manages
type settingsScope struct {
	OwnerID int64 // 0 for a repo or the instance
	RepoID  int64
	IsRepo  bool
	IsOrg   bool
	IsUser  bool
	IsAdmin bool
	// LinkPrefix is the page's ".../actions" settings link
	LinkPrefix string
}

func getSettingsScope(ctx *context.Context) (*settingsScope, error) {
	switch {
	case ctx.Data["PageIsRepoSettings"] == true:
		return &settingsScope{RepoID: ctx.Repo.Repository.ID, IsRepo: true, LinkPrefix: ctx.Repo.RepoLink + "/settings/actions"}, nil
	case ctx.Data["PageIsOrgSettings"] == true:
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			return nil, fmt.Errorf("getSettingsScope: %w", err)
		}
		return &settingsScope{OwnerID: ctx.Org.Organization.ID, IsOrg: true, LinkPrefix: ctx.Org.OrgLink + "/settings/actions"}, nil
	case ctx.Data["PageIsUserSettings"] == true:
		return &settingsScope{OwnerID: ctx.Doer.ID, IsUser: true, LinkPrefix: setting.AppSubURL + "/user/settings/actions"}, nil
	case ctx.Data["PageIsAdmin"] == true:
		return &settingsScope{IsAdmin: true, LinkPrefix: setting.AppSubURL + "/-/admin/actions"}, nil
	}
	return nil, errors.New("getSettingsScope: not an actions settings page")
}
