// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"

	"code.gitea.io/gitea/modules/markup"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func (r *HTMLRenderer) renderTaskCheckBoxListItem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*TaskCheckBoxListItem)
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<li")
			html.RenderAttributes(w, n, html.ListItemAttributeFilter)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<li>")
		}
		fmt.Fprintf(w, `<input type="checkbox" disabled="" data-source-position="%d"`, n.SourcePosition)
		if n.IsChecked {
			_, _ = w.WriteString(` checked=""`)
		}
		if r.XHTML {
			_, _ = w.WriteString(` />`)
		} else {
			_ = w.WriteByte('>')
		}
		fc := n.FirstChild()
		if fc != nil {
			if _, ok := fc.(*ast.TextBlock); !ok {
				_ = w.WriteByte('\n')
			}
		}
	} else {
		_, _ = w.WriteString("</li>\n")
	}
	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (g *ASTTransformer) transformList(_ *markup.RenderContext, v *ast.List, rc *RenderConfig) {
	if v.HasChildren() {
		children := make([]ast.Node, 0, v.ChildCount())
		child := v.FirstChild()
		for child != nil {
			children = append(children, child)
			child = child.NextSibling()
		}
		v.RemoveChildren(v)

		for _, child := range children {
			listItem := child.(*ast.ListItem)
			if !child.HasChildren() || !child.FirstChild().HasChildren() {
				v.AppendChild(v, child)
				continue
			}
			taskCheckBox, ok := child.FirstChild().FirstChild().(*east.TaskCheckBox)
			if !ok {
				v.AppendChild(v, child)
				continue
			}
			newChild := NewTaskCheckBoxListItem(listItem)
			newChild.IsChecked = taskCheckBox.IsChecked
			newChild.SetAttributeString("class", []byte("task-list-item"))
			segments := newChild.FirstChild().Lines()
			if segments.Len() > 0 {
				segment := segments.At(0)
				newChild.SourcePosition = rc.metaLength + segment.Start
			}
			v.AppendChild(v, newChild)
		}
	}
	g.applyElementDir(v)
}
