// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Markdown render markdown document to HTML
func Markdown(ctx *context.APIContext, form api.MarkdownOption) {
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
	if ctx.HasAPIError() {
		ctx.Error(422, "", ctx.GetErrMsg())
		return
	}

	if len(form.Text) == 0 {
		ctx.Write([]byte(""))
		return
	}

	switch form.Mode {
	case "gfm":
		md := []byte(form.Text)
		context := util.URLJoin(setting.AppURL, form.Context)
		if form.Wiki {
			ctx.Write([]byte(markdown.RenderWiki(md, context, nil)))
		} else {
			ctx.Write(markdown.Render(md, context, nil))
		}
	default:
		ctx.Write(markdown.RenderRaw([]byte(form.Text), "", false))
	}
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
	body, err := ctx.Req.Body().Bytes()
	if err != nil {
		ctx.Error(422, "", err)
		return
	}
	ctx.Write(markdown.RenderRaw(body, "", false))
}
