// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
)

// RequireRepoAdmin returns a middleware for requiring repository admin permission
func RequireRepoAdmin() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.IsSigned || !ctx.Repo.IsAdmin() {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
	}
}

// RequireRepoWriter returns a middleware for requiring repository write to the specify unitType
func RequireRepoWriter(unitType unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Repo.CanWrite(unitType) {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
	}
}

// CanEnableEditor checks if the user is allowed to write to the branch of the repo
func CanEnableEditor() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Repo.CanWriteToBranch(ctx.Doer, ctx.Repo.BranchName) {
			ctx.NotFound("CanWriteToBranch denies permission", nil)
			return
		}
	}
}

// RequireRepoWriterOr returns a middleware for requiring repository write to one of the unit permission
func RequireRepoWriterOr(unitTypes ...unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		for _, unitType := range unitTypes {
			if ctx.Repo.CanWrite(unitType) {
				return
			}
		}
		ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
	}
}

// RequireRepoReader returns a middleware for requiring repository read to the specify unitType
func RequireRepoReader(unitType unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Repo.CanRead(unitType) {
			if log.IsTrace() {
				if ctx.IsSigned {
					log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
						"User in Repo has Permissions: %-+v",
						ctx.Doer,
						unitType,
						ctx.Repo.Repository,
						ctx.Repo.Permission)
				} else {
					log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
						"Anonymous user in Repo has Permissions: %-+v",
						unitType,
						ctx.Repo.Repository,
						ctx.Repo.Permission)
				}
			}
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
	}
}

// RequireRepoReaderOr returns a middleware for requiring repository write to one of the unit permission
func RequireRepoReaderOr(unitTypes ...unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		for _, unitType := range unitTypes {
			if ctx.Repo.CanRead(unitType) {
				return
			}
		}
		if log.IsTrace() {
			var format string
			var args []interface{}
			if ctx.IsSigned {
				format = "Permission Denied: User %-v cannot read ["
				args = append(args, ctx.Doer)
			} else {
				format = "Permission Denied: Anonymous user cannot read ["
			}
			for _, unit := range unitTypes {
				format += "%-v, "
				args = append(args, unit)
			}

			format = format[:len(format)-2] + "] in Repo %-v\n" +
				"User in Repo has Permissions: %-+v"
			args = append(args, ctx.Repo.Repository, ctx.Repo.Permission)
			log.Trace(format, args...)
		}
		ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
	}
}

// RequireRepoScopedToken check whether personal access token has repo scope
func CheckRepoScopedToken(ctx *Context, repo *repo_model.Repository) {
	if !ctx.IsBasicAuth || ctx.Data["IsApiToken"] != true {
		return
	}

	var err error
	scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if ok { // it's a personal access token but not oauth2 token
		var scopeMatched bool
		scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopeRepo)
		if err != nil {
			ctx.ServerError("HasScope", err)
			return
		}
		if !scopeMatched && !repo.IsPrivate {
			scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopePublicRepo)
			if err != nil {
				ctx.ServerError("HasScope", err)
				return
			}
		}
		if !scopeMatched {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}
