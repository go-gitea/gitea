// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	api "github.com/go-gitea/go-sdk/gitea"

	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/modules/markdown"
)

// https://github.com/gogits/go-gogs-client/wiki/Miscellaneous#render-an-arbitrary-markdown-document
func Markdown(ctx *context.APIContext, form api.MarkdownOption) {
	if ctx.HasApiError() {
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

// https://github.com/gogits/go-gogs-client/wiki/Miscellaneous#render-a-markdown-document-in-raw-mode
func MarkdownRaw(ctx *context.APIContext) {
	body, err := ctx.Req.Body().Bytes()
	if err != nil {
		ctx.Error(422, "", err)
		return
	}
	ctx.Write(markdown.RenderRaw(body, ""))
}
