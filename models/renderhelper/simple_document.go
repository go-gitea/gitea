// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"

	"code.gitea.io/gitea/modules/markup"
)

type SimpleDocument struct {
	*markup.SimpleRenderHelper
	ctx      *markup.RenderContext
	baseLink string
}

func (r *SimpleDocument) ResolveLink(link, preferLinkType string) string {
	linkType, link := markup.ParseRenderedLink(link, preferLinkType)
	switch linkType {
	case markup.LinkTypeRoot:
		return r.ctx.ResolveLinkRoot(link)
	default:
		return r.ctx.ResolveLinkRelative(r.baseLink, "", link)
	}
}

var _ markup.RenderHelper = (*SimpleDocument)(nil)

func NewRenderContextSimpleDocument(ctx context.Context, baseLink string) *markup.RenderContext {
	helper := &SimpleDocument{baseLink: baseLink}
	rctx := markup.NewRenderContext(ctx).WithHelper(helper).WithMetas(markup.ComposeSimpleDocumentMetas())
	helper.ctx = rctx
	return rctx
}
