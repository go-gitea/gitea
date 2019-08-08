// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/indexer/issues"

	macaron "gopkg.in/macaron.v1"
)

// RebuildRepoIndex rebuilds a repository index
func RebuildRepoIndex(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	toobusy, err := models.RebuildRepoIndex(repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	if toobusy {
		ctx.PlainText(503, []byte("too busy"))
		return
	}

	ctx.PlainText(200, []byte("success"))
}

// RebuildIssueIndex rebuilds issue index
func RebuildIssueIndex(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	toobusy, err := issues.RebuildIssueIndex(repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	if toobusy {
		ctx.PlainText(503, []byte("too busy"))
		return
	}

	ctx.PlainText(200, []byte("success"))
}
