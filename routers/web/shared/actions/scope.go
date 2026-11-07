// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/setting"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

// settingsScope is the repo, owner or instance an actions settings page manages
type settingsScope struct {
	actions_model.BelongingScope
	OwnerID int64 // 0 for a repo or the instance
	RepoID  int64
	IsRepo  bool
	IsOrg   bool
	IsUser  bool
	IsAdmin bool
	// LinkPrefix is the page's ".../actions" settings link
	LinkPrefix string
}

func newSettingsScope(belonging actions_model.BelongingScope, linkPrefix string) *settingsScope {
	scope := &settingsScope{BelongingScope: belonging, LinkPrefix: linkPrefix}
	scope.OwnerID, scope.RepoID = belonging.GetOwnerRepoIDs()
	return scope
}

func getSettingsScope(ctx *context.Context) (*settingsScope, error) {
	switch {
	case ctx.Data["PageIsRepoSettings"] == true:
		scope := newSettingsScope(actions_model.BelongingScopeRepo(ctx.Repo.Repository.ID), ctx.Repo.RepoLink+"/settings/actions")
		scope.IsRepo = true
		return scope, nil
	case ctx.Data["PageIsOrgSettings"] == true:
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			return nil, fmt.Errorf("getSettingsScope: %w", err)
		}
		scope := newSettingsScope(actions_model.BelongingScopeOwner(ctx.Org.Organization.ID), ctx.Org.OrgLink+"/settings/actions")
		scope.IsOrg = true
		return scope, nil
	case ctx.Data["PageIsUserSettings"] == true:
		scope := newSettingsScope(actions_model.BelongingScopeOwner(ctx.Doer.ID), setting.AppSubURL+"/user/settings/actions")
		scope.IsUser = true
		return scope, nil
	case ctx.Data["PageIsAdmin"] == true:
		scope := newSettingsScope(actions_model.BelongingScopeGlobal(), setting.AppSubURL+"/-/admin/actions")
		scope.IsAdmin = true
		return scope, nil
	}
	return nil, errors.New("getSettingsScope: not an actions settings page")
}
