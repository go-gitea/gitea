// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markdown"
)

// Markdown render markdown document to HTML
// see https://github.com/gogits/go-gogs-client/wiki/Miscellaneous#render-an-arbitrary-markdown-document
func Markdown(ctx *context.APIContext, form api.MarkdownOption) {
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
		ctx.Write(markdown.Render([]byte(form.Text), form.Context, nil))
	default:
		ctx.Write(markdown.RenderRaw([]byte(form.Text), ""))
	}
}

// MarkdownRaw render raw markdown HTML
// see https://github.com/gogits/go-gogs-client/wiki/Miscellaneous#render-a-markdown-document-in-raw-mode
func MarkdownRaw(ctx *context.APIContext) {
	body, err := ctx.Req.Body().Bytes()
	if err != nil {
		ctx.Error(422, "", err)
		return
	}
	ctx.Write(markdown.RenderRaw(body, ""))
}
