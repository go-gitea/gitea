// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"gitea.com/macaron/macaron"
)

// RequireRepoAdmin returns a macaron middleware for requiring repository admin permission
func RequireRepoAdmin() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsSigned || !ctx.Repo.IsAdmin() {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
	}
}

// RequireRepoWriter returns a macaron middleware for requiring repository write to the specify unitType
func RequireRepoWriter(unitType models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		if !ctx.Repo.CanWrite(unitType) {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
	}
}

// RequireRepoWriterOr returns a macaron middleware for requiring repository write to one of the unit permission
func RequireRepoWriterOr(unitTypes ...models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		for _, unitType := range unitTypes {
			if ctx.Repo.CanWrite(unitType) {
				return
			}
		}
		ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
	}
}

// RequireRepoReader returns a macaron middleware for requiring repository read to the specify unitType
func RequireRepoReader(unitType models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		if !ctx.Repo.CanRead(unitType) {
			if log.IsTrace() {
				if ctx.IsSigned {
					log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
						"User in Repo has Permissions: %-+v",
						ctx.User,
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

// RequireRepoReaderOr returns a macaron middleware for requiring repository write to one of the unit permission
func RequireRepoReaderOr(unitTypes ...models.UnitType) macaron.Handler {
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
				args = append(args, ctx.User)
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
