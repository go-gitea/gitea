// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"

	"code.gitea.io/gitea/modules/markup"
)

type Application struct {
	ctx *markup.RenderContext
}

func (r *Application) CleanUp() {
}

func (r *Application) IsCommitIDExisting(commitID string) bool {
	return false
}

func (r *Application) ResolveLink(link, preferLinkType string) (finalLink string) {
	linkType, link := markup.ParseRenderedLink(link, preferLinkType)
	switch linkType {
	case markup.LinkTypeRoot:
		finalLink = r.ctx.ResolveLinkRoot(link)
	}
	return finalLink
}

var _ markup.RenderHelper = (*Application)(nil)

func NewRenderContextApplication(ctx context.Context) *markup.RenderContext {
	helper := &Application{}
	rctx := markup.NewRenderContext(ctx)
	helper.ctx = rctx

	rctx = rctx.WithHelper(helper)
	return rctx
}
