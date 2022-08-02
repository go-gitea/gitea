// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package math

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Extension is a math extension
type Extension struct{}

// Option is the interface Options should implement
type Option interface {
	SetOption(e *Extension)
}

type extensionFunc func(e *Extension)

func (fn extensionFunc) SetOption(e *Extension) {
	fn(e)
}

// Math represents a math extension with default rendered delimiters
var Math = &Extension{}

// NewExtension creates a new math extension with the provided options
func NewExtension(opts ...Option) *Extension {
	r := &Extension{}

	for _, o := range opts {
		o.SetOption(r)
	}
	return r
}

// Extend extends goldmark with our parsers and renderers
func (e *Extension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(NewBlockParser(), 701),
	))

	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(NewInlineBracketParser(), 501),
		util.Prioritized(NewInlineDollarParser(), 501),
	))

	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewBlockRenderer(), 501),
		util.Prioritized(NewInlineRenderer(), 502),
	))
}
