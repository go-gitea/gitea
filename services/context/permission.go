// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"net/http"
	"slices"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	module_auth "gitea.dev/modules/auth"
	"gitea.dev/modules/reqctx"
)

// isPublicRepository applies anonymous visibility to ungranted Codespace reads.
func isPublicRepository(ctx context.Context, repo *repo_model.Repository) bool {
	return repo != nil && !repo.IsPrivate && repo.LoadOwner(ctx) == nil && repo.Owner != nil && repo.Owner.Visibility.IsPublic()
}

type codespaceTokenRepoSnapshot interface {
	CodespaceTokenRepoID() int64
	CodespaceTokenAllowsAnyRepository(repoID int64) bool
	CodespaceTokenAllowsRepository(repoID int64, unitType unit.Type, mode perm.AccessMode) bool
}

func codespaceTokenSnapshotFromData(data reqctx.ContextData) (codespaceTokenRepoSnapshot, bool) {
	snapshot, ok := data[module_auth.CodespaceTokenAuthDataKey].(codespaceTokenRepoSnapshot)
	if !ok {
		return nil, false
	}
	return snapshot, true
}

// CheckTokenScopes checks whether the authenticated API token contains any of the given scopes.
func CheckTokenScopes(ctx *Context, scopes ...auth_model.AccessTokenScope) {
	scope, hasApiTokenScope := user_model.GetDoerTokenScope(ctx.Doer)
	if !hasApiTokenScope {
		return
	}

	scopeMatched, err := scope.HasAnyScope(scopes...)
	if err != nil {
		ctx.ServerError("HasAnyScope", err)
		return
	}

	if !scopeMatched {
		ctx.HTTPError(http.StatusForbidden)
	}
}

// RequireRepoAdmin returns a middleware for requiring repository admin permission
func RequireRepoAdmin() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.IsSigned || !ctx.Repo.Permission.IsAdmin() {
			ctx.NotFound(nil)
			return
		}
	}
}

// CanWriteToBranch checks if the user is allowed to write to the branch of the repo
func CanWriteToBranch() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName) {
			ctx.NotFound(nil)
			return
		}
	}
}

// RequireUnitWriter returns a middleware for requiring repository write to one of the unit permission
func RequireUnitWriter(unitTypes ...unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		if slices.ContainsFunc(unitTypes, ctx.Repo.Permission.CanWrite) {
			return
		}
		ctx.NotFound(nil)
	}
}

// RequireUnitReader returns a middleware for requiring repository write to one of the unit permission
func RequireUnitReader(unitTypes ...unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		for _, unitType := range unitTypes {
			if ctx.Repo.Permission.CanRead(unitType) {
				return
			}
			if unitType == unit.TypeCode && canWriteAsMaintainer(ctx) {
				return
			}
		}
		ctx.NotFound(nil)
	}
}

// CheckRepoScopedToken checks whether the authenticated API token has repo scope.
func CheckRepoScopedToken(ctx *Context, repo *repo_model.Repository, unitType unit.Type, level auth_model.AccessTokenScopeLevel) {
	if snapshot, ok := codespaceTokenSnapshotFromData(ctx.Data); ok {
		mode := perm.AccessModeRead
		if level == auth_model.Write {
			mode = perm.AccessModeWrite
		}
		allowed := repo != nil && snapshot.CodespaceTokenAllowsRepository(repo.ID, unitType, mode)
		if !allowed && mode == perm.AccessModeRead && repo != nil {
			allowed = isPublicRepository(ctx, repo)
		}
		if !allowed {
			ctx.HTTPError(http.StatusForbidden)
			return
		}
	}
	CheckTokenScopes(ctx, auth_model.GetRequiredScopes(level, auth_model.AccessTokenScopeCategoryRepository)...)
}
