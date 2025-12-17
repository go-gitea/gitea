// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"slices"
	"strconv"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const tplRepoSettingsPublicAccess templates.TplName = "repo/settings/public_access"

func parsePublicAccessMode(permission string, allowed []string) (ret struct {
	AnonymousAccessMode, EveryoneAccessMode perm.AccessMode
},
) {
	ret.AnonymousAccessMode = perm.AccessModeNone
	ret.EveryoneAccessMode = perm.AccessModeNone

	// if site admin forces repositories to be private, then do not allow any other access mode,
	// otherwise the "force private" setting would be bypassed
	if setting.Repository.ForcePrivate {
		return ret
	}
	if !slices.Contains(allowed, permission) {
		return ret
	}
	switch permission {
	case paAnonymousRead:
		ret.AnonymousAccessMode = perm.AccessModeRead
	case paEveryoneRead:
		ret.EveryoneAccessMode = perm.AccessModeRead
	case paEveryoneWrite:
		ret.EveryoneAccessMode = perm.AccessModeWrite
	}
	return ret
}

const (
	paNotSet        = "not-set"
	paAnonymousRead = "anonymous-read"
	paEveryoneRead  = "everyone-read"
	paEveryoneWrite = "everyone-write"
)

type repoUnitPublicAccess struct {
	UnitType          unit.Type
	FormKey           string
	DisplayName       string
	PublicAccessTypes []string
	UnitPublicAccess  string
}

func repoUnitPublicAccesses(ctx *context.Context) []*repoUnitPublicAccess {
	accesses := []*repoUnitPublicAccess{
		{
			UnitType:          unit.TypeCode,
			DisplayName:       ctx.Locale.TrString("repo.code"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
		{
			UnitType:          unit.TypeIssues,
			DisplayName:       ctx.Locale.TrString("issues"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
		{
			UnitType:          unit.TypePullRequests,
			DisplayName:       ctx.Locale.TrString("pull_requests"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
		{
			UnitType:          unit.TypeReleases,
			DisplayName:       ctx.Locale.TrString("repo.releases"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
		{
			UnitType:          unit.TypeWiki,
			DisplayName:       ctx.Locale.TrString("repo.wiki"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead, paEveryoneWrite},
		},
		{
			UnitType:          unit.TypeProjects,
			DisplayName:       ctx.Locale.TrString("repo.projects"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
		{
			UnitType:          unit.TypePackages,
			DisplayName:       ctx.Locale.TrString("repo.packages"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
		{
			UnitType:          unit.TypeActions,
			DisplayName:       ctx.Locale.TrString("repo.actions"),
			PublicAccessTypes: []string{paAnonymousRead, paEveryoneRead},
		},
	}
	for _, ua := range accesses {
		ua.FormKey = "repo-unit-access-" + strconv.Itoa(int(ua.UnitType))
		for _, u := range ctx.Repo.Repository.Units {
			if u.Type == ua.UnitType {
				ua.UnitPublicAccess = paNotSet
				switch {
				case u.EveryoneAccessMode == perm.AccessModeWrite:
					ua.UnitPublicAccess = paEveryoneWrite
				case u.EveryoneAccessMode == perm.AccessModeRead:
					ua.UnitPublicAccess = paEveryoneRead
				case u.AnonymousAccessMode == perm.AccessModeRead:
					ua.UnitPublicAccess = paAnonymousRead
				}
				break
			}
		}
	}
	return slices.DeleteFunc(accesses, func(ua *repoUnitPublicAccess) bool {
		return ua.UnitPublicAccess == ""
	})
}

func PublicAccess(ctx *context.Context) {
	ctx.Data["PageIsSettingsPublicAccess"] = true
	ctx.Data["RepoUnitPublicAccesses"] = repoUnitPublicAccesses(ctx)
	ctx.Data["GlobalForcePrivate"] = setting.Repository.ForcePrivate
	if setting.Repository.ForcePrivate {
		ctx.Flash.Error(ctx.Tr("form.repository_force_private"), true)
	}
	ctx.HTML(http.StatusOK, tplRepoSettingsPublicAccess)
}

func PublicAccessPost(ctx *context.Context) {
	accesses := repoUnitPublicAccesses(ctx)
	for _, ua := range accesses {
		formVal := ctx.FormString(ua.FormKey)
		parsed := parsePublicAccessMode(formVal, ua.PublicAccessTypes)
		err := repo.UpdateRepoUnitPublicAccess(ctx, &repo.RepoUnit{
			RepoID:              ctx.Repo.Repository.ID,
			Type:                ua.UnitType,
			AnonymousAccessMode: parsed.AnonymousAccessMode,
			EveryoneAccessMode:  parsed.EveryoneAccessMode,
		})
		if err != nil {
			ctx.ServerError("UpdateRepoUnitPublicAccess", err)
			return
		}
	}
	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(ctx.Repo.Repository.Link() + "/settings/public_access")
}
