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

type settingsScopeKind int

const (
	settingsScopeRepo settingsScopeKind = iota + 1
	settingsScopeOrg
	settingsScopeUser
	settingsScopeAdmin
)

// settingsScope is the repo, owner or instance an actions settings page manages
type settingsScope struct {
	Kind    settingsScopeKind
	OwnerID int64 // 0 for a repo or the instance
	RepoID  int64
	// LinkPrefix is the page's ".../actions" settings link
	LinkPrefix string
}

func (s *settingsScope) IsRepo() bool  { return s.Kind == settingsScopeRepo }
func (s *settingsScope) IsOrg() bool   { return s.Kind == settingsScopeOrg }
func (s *settingsScope) IsUser() bool  { return s.Kind == settingsScopeUser }
func (s *settingsScope) IsAdmin() bool { return s.Kind == settingsScopeAdmin }

func (s *settingsScope) BelongingScope() actions_model.BelongingScope {
	switch s.Kind {
	case settingsScopeRepo:
		return actions_model.BelongingScopeRepo(s.RepoID)
	case settingsScopeOrg, settingsScopeUser:
		return actions_model.BelongingScopeOwner(s.OwnerID)
	}
	return actions_model.BelongingScopeGlobal()
}

func getSettingsScope(ctx *context.Context) (*settingsScope, error) {
	switch {
	case ctx.Data["PageIsRepoSettings"] == true:
		return &settingsScope{Kind: settingsScopeRepo, RepoID: ctx.Repo.Repository.ID, LinkPrefix: ctx.Repo.RepoLink + "/settings/actions"}, nil
	case ctx.Data["PageIsOrgSettings"] == true:
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			return nil, fmt.Errorf("getSettingsScope: %w", err)
		}
		return &settingsScope{Kind: settingsScopeOrg, OwnerID: ctx.Org.Organization.ID, LinkPrefix: ctx.Org.OrgLink + "/settings/actions"}, nil
	case ctx.Data["PageIsUserSettings"] == true:
		return &settingsScope{Kind: settingsScopeUser, OwnerID: ctx.Doer.ID, LinkPrefix: setting.AppSubURL + "/user/settings/actions"}, nil
	case ctx.Data["PageIsAdmin"] == true:
		return &settingsScope{Kind: settingsScopeAdmin, LinkPrefix: setting.AppSubURL + "/-/admin/actions"}, nil
	}
	return nil, errors.New("getSettingsScope: not an actions settings page")
}
