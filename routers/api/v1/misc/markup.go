// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"

	"mvdan.cc/xurls/v2"
)

// Render markup text in given mode
func renderMarkup(ctx *context.APIContext, mode, text, urlPrefix, filePath string, wiki bool) {
	markupType := ""
	relativePath := ""

	if len(text) == 0 {
		_, _ = ctx.Write([]byte(""))
		return
	}

	if mode == "markdown" {
		// Raw markdown
		if err := markdown.RenderRaw(&markup.RenderContext{
			Ctx:       ctx,
			URLPrefix: urlPrefix,
		}, strings.NewReader(text), ctx.Resp); err != nil {
			ctx.InternalServerError(err)
		}
		return
	} else if mode == "comment" {
		// Comment as markdown
		markupType = markdown.MarkupName
	} else if mode == "gfm" {
		// Github Flavored Markdown as document
		markupType = markdown.MarkupName
	} else if mode == "file" {
		// File as document based on file extension
		markupType = ""
		relativePath = filePath
	} else {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Unknown mode: %s", mode))
		return
	}

	if !strings.HasPrefix(setting.AppSubURL+"/", urlPrefix) {
		// check if urlPrefix is already set to a URL
		linkRegex, _ := xurls.StrictMatchingScheme("https?://")
		m := linkRegex.FindStringIndex(urlPrefix)
		if m == nil {
			urlPrefix = util.URLJoin(setting.AppURL, urlPrefix)
		}
	}

	meta := map[string]string{}
	if ctx.Repo != nil && ctx.Repo.Repository != nil {
		if mode == "comment" {
			meta = ctx.Repo.Repository.ComposeMetas()
		} else {
			meta = ctx.Repo.Repository.ComposeDocumentMetas()
		}
	}
	if mode != "comment" {
		meta["mode"] = "document"
	}

	if err := markup.Render(&markup.RenderContext{
		Ctx:          ctx,
		URLPrefix:    urlPrefix,
		Metas:        meta,
		IsWiki:       wiki,
		Type:         markupType,
		RelativePath: relativePath,
	}, strings.NewReader(text), ctx.Resp); err != nil {
		if markup.IsErrUnsupportedRenderExtension(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, "", err.Error())
		}
		return
	}
}

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

	renderMarkup(ctx, form.Mode, form.Text, form.Context, form.FilePath, form.Wiki)
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

	renderMarkup(ctx, mode, form.Text, form.Context, "", form.Wiki)
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
