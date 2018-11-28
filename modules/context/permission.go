// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"code.gitea.io/gitea/models"
	macaron "gopkg.in/macaron.v1"
)

// RequireRepoAdmin returns a macaron middleware for requiring repository admin permission
func RequireRepoAdmin() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsSigned || !ctx.Repo.IsAdmin() {
			ctx.NotFound(ctx.Req.RequestURI, nil)
			return
		}
	}
}

// RequireRepoWriter returns a macaron middleware for requiring repository write to the specify unitType
func RequireRepoWriter(unitType models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		if !ctx.Repo.CanWrite(unitType) {
			ctx.NotFound(ctx.Req.RequestURI, nil)
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
		ctx.NotFound(ctx.Req.RequestURI, nil)
	}
}

// RequireRepoReader returns a macaron middleware for requiring repository read to the specify unitType
func RequireRepoReader(unitType models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		if !ctx.Repo.CanRead(unitType) {
			ctx.NotFound(ctx.Req.RequestURI, nil)
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
		ctx.NotFound(ctx.Req.RequestURI, nil)
	}
}
