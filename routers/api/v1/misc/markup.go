// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
)

// Markup render markup document to HTML
func Markup(ctx *context.APIContext) {
	// swagger:operation POST /markup miscellaneous renderMarkup
	// ---
	// summary: Render a markup document as HTML
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MarkupOption"
	// consumes:
	// - application/json
	// produces:
	//     - text/html
	// responses:
	//   "200":
	//     "$ref": "#/responses/MarkupRender"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.MarkupOption)

	if ctx.HasAPIError() {
		ctx.Error(http.StatusUnprocessableEntity, "", ctx.GetErrMsg())
		return
	}

	common.RenderMarkup(ctx.Base, ctx.Repo, form.Mode, form.Text, form.Context, form.FilePath, form.Wiki)
}

// Markdown render markdown document to HTML
func Markdown(ctx *context.APIContext) {
	// swagger:operation POST /markdown miscellaneous renderMarkdown
	// ---
	// summary: Render a markdown document as HTML
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MarkdownOption"
	// consumes:
	// - application/json
	// produces:
	//     - text/html
	// responses:
	//   "200":
	//     "$ref": "#/responses/MarkdownRender"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.MarkdownOption)

	if ctx.HasAPIError() {
		ctx.Error(http.StatusUnprocessableEntity, "", ctx.GetErrMsg())
		return
	}

	mode := "markdown"
	if form.Mode == "comment" || form.Mode == "gfm" {
		mode = form.Mode
	}

	common.RenderMarkup(ctx.Base, ctx.Repo, mode, form.Text, form.Context, "", form.Wiki)
}

// MarkdownRaw render raw markdown HTML
func MarkdownRaw(ctx *context.APIContext) {
	// swagger:operation POST /markdown/raw miscellaneous renderMarkdownRaw
	// ---
	// summary: Render raw markdown as HTML
	// parameters:
	//     - name: body
	//       in: body
	//       description: Request body to render
	//       required: true
	//       schema:
	//         type: string
	// consumes:
	//     - text/plain
	// produces:
	//     - text/html
	// responses:
	//   "200":
	//     "$ref": "#/responses/MarkdownRender"
	//   "422":
	//     "$ref": "#/responses/validationError"
	defer ctx.Req.Body.Close()
	if err := markdown.RenderRaw(&markup.RenderContext{
		Ctx: ctx,
	}, ctx.Req.Body, ctx.Resp); err != nil {
		ctx.InternalServerError(err)
		return
	}
}
