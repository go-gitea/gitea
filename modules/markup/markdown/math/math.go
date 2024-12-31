// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	"code.gitea.io/gitea/modules/markup/internal"
	giteaUtil "code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type Options struct {
	Enabled           bool
	ParseDollarInline bool
	ParseDollarBlock  bool
	ParseSquareBlock  bool
}

// Extension is a math extension
type Extension struct {
	renderInternal *internal.RenderInternal
	options        Options
}

// NewExtension creates a new math extension with the provided options
func NewExtension(renderInternal *internal.RenderInternal, opts ...Options) *Extension {
	opt := giteaUtil.OptionalArg(opts)
	r := &Extension{
		renderInternal: renderInternal,
		options:        opt,
	}
	return r
}

// Extend extends goldmark with our parsers and renderers
func (e *Extension) Extend(m goldmark.Markdown) {
	if !e.options.Enabled {
		return
	}

	inlines := []util.PrioritizedValue{util.Prioritized(NewInlineBracketParser(), 501)}
	if e.options.ParseDollarInline {
		inlines = append(inlines, util.Prioritized(NewInlineDollarParser(), 502))
	}
	m.Parser().AddOptions(parser.WithInlineParsers(inlines...))

	m.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(NewBlockParser(e.options.ParseDollarBlock, e.options.ParseSquareBlock), 701),
	))

	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewBlockRenderer(e.renderInternal), 501),
		util.Prioritized(NewInlineRenderer(e.renderInternal), 502),
	))
}
