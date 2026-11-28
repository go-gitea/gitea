// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

type settingsScopeKind int

const (
	settingsScopeRepo settingsScopeKind = iota + 1
	settingsScopeOrg
	settingsScopeUser
	settingsScopeAdmin
)

type settingsScope struct {
	Kind            settingsScopeKind
	OwnerID, RepoID int64
	BelongingScope  actions_model.BelongingScope
	LinkPrefix      string
	Template        templates.TplName // TODO: Separate secrets from runners when layout is ready
}

func (s *settingsScope) IsAdmin() bool { return s.Kind == settingsScopeAdmin }

func getSettingsScope(ctx *context.Context) (*settingsScope, error) {
	scope := &settingsScope{}
	switch {
	case ctx.Data["PageIsRepoSettings"] == true:
		scope.Kind, scope.BelongingScope = settingsScopeRepo, actions_model.BelongingScopeRepo(ctx.Repo.Repository.ID)
		scope.LinkPrefix, scope.Template = ctx.Repo.RepoLink+"/settings/actions", "repo/settings/actions"
	case ctx.Data["PageIsOrgSettings"] == true:
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			return nil, fmt.Errorf("getSettingsScope: %w", err)
		}
		scope.Kind, scope.BelongingScope = settingsScopeOrg, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID)
		scope.LinkPrefix, scope.Template = ctx.Org.OrgLink+"/settings/actions", "org/settings/actions"
	case ctx.Data["PageIsUserSettings"] == true:
		scope.Kind, scope.BelongingScope = settingsScopeUser, actions_model.BelongingScopeOwner(ctx.Doer.ID)
		scope.LinkPrefix, scope.Template = setting.AppSubURL+"/user/settings/actions", "user/settings/actions"
	case ctx.Data["PageIsAdmin"] == true:
		scope.Kind, scope.BelongingScope = settingsScopeAdmin, actions_model.BelongingScopeGlobal()
		scope.LinkPrefix, scope.Template = setting.AppSubURL+"/-/admin/actions", "admin/actions"
	default:
		return nil, errors.New("getSettingsScope: not an actions settings page")
	}
	scope.OwnerID, scope.RepoID = scope.BelongingScope.GetOwnerRepoIDs()
	return scope, nil
}
