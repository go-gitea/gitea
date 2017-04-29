// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
)

// Markdown render markdown document to HTML
func Markdown(ctx *context.APIContext, form api.MarkdownOption) {
	// swagger:route POST /markdown renderMarkdown
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - text/html
	//
	//     Responses:
	//       200: MarkdownRender
	//       422: validationError

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
		context := markdown.URLJoin(setting.AppURL, form.Context)
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
	// swagger:route POST /markdown/raw renderMarkdownRaw
	//
	//     Consumes:
	//     - text/plain
	//
	//     Produces:
	//     - text/html
	//
	//     Responses:
	//       200: MarkdownRender
	//       422: validationError
	body, err := ctx.Req.Body().Bytes()
	if err != nil {
		ctx.Error(422, "", err)
		return
	}
	ctx.Write(markdown.RenderRaw(body, "", false))
}
