// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"slices"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
)

// CheckTokenScopes checks whether the authenticated API token contains any of the given scopes.
func CheckTokenScopes(ctx *Context, repo *repo_model.Repository, scopes ...auth_model.AccessTokenScope) {
	if ctx.Data["IsApiToken"] != true {
		return
	}

	scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if !ok {
		return
	}

	publicOnly, err := scope.PublicOnly()
	if err != nil {
		ctx.ServerError("PublicOnly", err)
		return
	}

	if publicOnly && repo != nil && repo.IsPrivate {
		ctx.HTTPError(http.StatusForbidden)
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
func CheckRepoScopedToken(ctx *Context, repo *repo_model.Repository, level auth_model.AccessTokenScopeLevel) {
	CheckTokenScopes(ctx, repo, auth_model.GetRequiredScopes(level, auth_model.AccessTokenScopeCategoryRepository)...)
}
