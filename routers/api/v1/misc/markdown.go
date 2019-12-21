// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"mvdan.cc/xurls/v2"
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
		ctx.Error(http.StatusUnprocessableEntity, "", ctx.GetErrMsg())
		return
	}

	if len(form.Text) == 0 {
		_, _ = ctx.Write([]byte(""))
		return
	}

	switch form.Mode {
	case "gfm":
		md := []byte(form.Text)
		urlPrefix := form.Context
		var meta map[string]string
		if !strings.HasPrefix(setting.AppSubURL+"/", urlPrefix) {
			// check if urlPrefix is already set to a URL
			linkRegex, _ := xurls.StrictMatchingScheme("https?://")
			m := linkRegex.FindStringIndex(urlPrefix)
			if m == nil {
				urlPrefix = util.URLJoin(setting.AppURL, form.Context)
			}
		}
		if ctx.Repo != nil && ctx.Repo.Repository != nil {
			meta = ctx.Repo.Repository.ComposeMetas()
		}
		if form.Wiki {
			_, err := ctx.Write([]byte(markdown.RenderWiki(md, urlPrefix, meta)))
			if err != nil {
				ctx.InternalServerError(err)
				return
			}
		} else {
			_, err := ctx.Write(markdown.Render(md, urlPrefix, meta))
			if err != nil {
				ctx.InternalServerError(err)
				return
			}
		}
	default:
		_, err := ctx.Write(markdown.RenderRaw([]byte(form.Text), "", false))
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
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
		ctx.Error(http.StatusUnprocessableEntity, "", err)
		return
	}
	_, err = ctx.Write(markdown.RenderRaw(body, "", false))
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
}
