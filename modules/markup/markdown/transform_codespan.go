// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/markup"

	"github.com/microcosm-cc/bluemonday/css"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// renderCodeSpan renders CodeSpan elements (like goldmark upstream does) but also renders ColorPreview elements.
// See #21474 for reference
func (r *HTMLRenderer) renderCodeSpan(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<code")
			html.RenderAttributes(w, n, html.CodeAttributeFilter)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<code>")
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch v := c.(type) {
			case *ast.Text:
				segment := v.Segment
				value := segment.Value(source)
				if bytes.HasSuffix(value, []byte("\n")) {
					r.Writer.RawWrite(w, value[:len(value)-1])
					r.Writer.RawWrite(w, []byte(" "))
				} else {
					r.Writer.RawWrite(w, value)
				}
			case *ColorPreview:
				_, _ = w.WriteString(fmt.Sprintf(`<span class="color-preview" style="background-color: %v"></span>`, string(v.Color)))
			}
		}
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString("</code>")
	return ast.WalkContinue, nil
}

func (g *ASTTransformer) transformCodeSpan(ctx *markup.RenderContext, v *ast.CodeSpan, reader text.Reader) {
	colorContent := v.Text(reader.Source())
	if css.ColorHandler(strings.ToLower(string(colorContent))) {
		v.AppendChild(v, NewColorPreview(colorContent))
	}
}
