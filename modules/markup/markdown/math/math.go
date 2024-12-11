// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	"code.gitea.io/gitea/modules/markup/internal"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Extension is a math extension
type Extension struct {
	renderInternal    *internal.RenderInternal
	enabled           bool
	parseDollarInline bool
	parseDollarBlock  bool
}

// Option is the interface Options should implement
type Option interface {
	SetOption(e *Extension)
}

type extensionFunc func(e *Extension)

func (fn extensionFunc) SetOption(e *Extension) {
	fn(e)
}

// Enabled enables or disables this extension
func Enabled(enable ...bool) Option {
	value := true
	if len(enable) > 0 {
		value = enable[0]
	}
	return extensionFunc(func(e *Extension) {
		e.enabled = value
	})
}

// NewExtension creates a new math extension with the provided options
func NewExtension(renderInternal *internal.RenderInternal, opts ...Option) *Extension {
	r := &Extension{
		renderInternal:    renderInternal,
		enabled:           true,
		parseDollarBlock:  true,
		parseDollarInline: true,
	}

	for _, o := range opts {
		o.SetOption(r)
	}
	return r
}

// Extend extends goldmark with our parsers and renderers
func (e *Extension) Extend(m goldmark.Markdown) {
	if !e.enabled {
		return
	}

	m.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(NewBlockParser(e.parseDollarBlock), 701),
	))

	inlines := []util.PrioritizedValue{
		util.Prioritized(NewInlineBracketParser(), 501),
	}
	if e.parseDollarInline {
		inlines = append(inlines, util.Prioritized(NewInlineDollarParser(), 503),
			util.Prioritized(NewInlineDualDollarParser(), 502))
	}
	m.Parser().AddOptions(parser.WithInlineParsers(inlines...))

	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewBlockRenderer(e.renderInternal), 501),
		util.Prioritized(NewInlineRenderer(e.renderInternal), 502),
	))
}
