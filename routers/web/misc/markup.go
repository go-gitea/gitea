// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"cmp"

	"gitea.dev/modules/markup"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/common"
	"gitea.dev/services/context"
)

// Markup render markup document to HTML
func Markup(ctx *context.Context) {
	form := web.GetForm[*api.MarkupOption](ctx)
	mode := util.Iif(form.Wiki, "wiki", form.Mode) //nolint:staticcheck // form.Wiki is deprecated
	var anchor *markup.CodeSuggestionAnchor
	line := ctx.FormInt64("code_suggestion_line")
	startLine := cmp.Or(ctx.FormInt64("code_suggestion_start_line"), line)
	if mode == "comment" && startLine > 0 && line >= startLine &&
		ctx.FormString("code_suggestion_side", "proposed") == "proposed" && ctx.FormString("code_suggestion_start_side", "proposed") == "proposed" {
		anchor = &markup.CodeSuggestionAnchor{
			CommitID:  ctx.FormString("code_suggestion_commit_id"),
			FilePath:  ctx.FormString("code_suggestion_path"),
			StartLine: uint64(startLine),
			EndLine:   uint64(line),
		}
	}
	common.RenderMarkup(ctx.Base, ctx.Repo, mode, form.Text, form.Context, form.FilePath, anchor)
}
