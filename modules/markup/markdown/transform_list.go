// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strconv"

	"gitea.dev/modules/markup"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func (r *HTMLRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	checkBox, ok := node.(*east.TaskCheckBox)
	if !entering || !ok {
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<input type="checkbox" disabled=""`)
	html.RenderAttributes(w, checkBox, nil)
	if checkBox.IsChecked {
		_, _ = w.WriteString(` checked=""`)
	}
	if r.XHTML {
		_, _ = w.WriteString(` />`)
	} else {
		_ = w.WriteByte('>')
	}
	return ast.WalkContinue, nil
}

func (g *ASTTransformer) transformList(_ *markup.RenderContext, v *ast.List, rc *RenderConfig) {
	for listItem := v.FirstChild(); listItem != nil; listItem = listItem.NextSibling() {
		block := listItem.FirstChild()
		if block == nil {
			continue
		}
		checkBox, ok := block.FirstChild().(*east.TaskCheckBox)
		if !ok {
			continue
		}
		listItem.SetAttributeString(g.renderInternal.SafeAttr("class"), []byte(g.renderInternal.SafeValue("task-list-item")))
		if lines := block.Lines(); lines.Len() > 0 {
			checkBox.SetAttributeString("data-source-position", []byte(strconv.Itoa(rc.metaLength+lines.At(0).Start)))
		}
	}

	nestedList := false
	for p := v.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.List); ok {
			nestedList = true
			break
		}
	}
	if !nestedList {
		// "dir=auto" should be only added to top-level "ul". https://github.com/go-gitea/gitea/issues/35058
		g.applyElementDir(v)
	}
}
