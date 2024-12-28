// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	"bytes"

	"code.gitea.io/gitea/modules/markup/internal"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Inline render output:
// <code class="language-math">...</code>

// InlineRenderer is an inline renderer
type InlineRenderer struct {
	renderInternal *internal.RenderInternal
}

// NewInlineRenderer returns a new renderer for inline math
func NewInlineRenderer(renderInternal *internal.RenderInternal) renderer.NodeRenderer {
	return &InlineRenderer{renderInternal: renderInternal}
}

func (r *InlineRenderer) renderInline(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_ = r.renderInternal.FormatWithSafeAttrs(w, `<code class="language-math">`)
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			segment := c.(*ast.Text).Segment
			value := util.EscapeHTML(segment.Value(source))
			if bytes.HasSuffix(value, []byte("\n")) {
				_, _ = w.Write(value[:len(value)-1])
				if c != n.LastChild() {
					_, _ = w.Write([]byte(" "))
				}
			} else {
				_, _ = w.Write(value)
			}
		}
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString(`</code>`)
	return ast.WalkContinue, nil
}

// RegisterFuncs registers the renderer for inline math nodes
func (r *InlineRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindInline, r.renderInline)
}
