// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (g *ASTTransformer) transformBlockquote(v *ast.Blockquote, reader text.Reader) (ast.WalkStatus, error) {
	// We only want attention blockquotes when the AST looks like:
	// > Text("[") Text("!TYPE") Text("]")

	// grab these nodes and make sure we adhere to the attention blockquote structure
	firstParagraph := v.FirstChild()
	g.applyElementDir(firstParagraph)
	if firstParagraph.ChildCount() < 3 {
		return ast.WalkContinue, nil
	}
	node1, ok1 := firstParagraph.FirstChild().(*ast.Text)
	node2, ok2 := node1.NextSibling().(*ast.Text)
	node3, ok3 := node2.NextSibling().(*ast.Text)
	if !ok1 || !ok2 || !ok3 {
		return ast.WalkContinue, nil
	}
	val1 := string(node1.Segment.Value(reader.Source()))
	val2 := string(node2.Segment.Value(reader.Source()))
	val3 := string(node3.Segment.Value(reader.Source()))
	if val1 != "[" || val3 != "]" || !strings.HasPrefix(val2, "!") {
		return ast.WalkContinue, nil
	}

	// grab attention type from markdown source
	attentionType := strings.ToLower(val2[1:])
	if !g.AttentionTypes.Contains(attentionType) {
		return ast.WalkContinue, nil
	}

	// color the blockquote
	v.SetAttributeString("class", []byte("attention-header attention-"+attentionType))

	// create an emphasis to make it bold
	attentionParagraph := ast.NewParagraph()
	g.applyElementDir(attentionParagraph)
	emphasis := ast.NewEmphasis(2)
	emphasis.SetAttributeString("class", []byte("attention-"+attentionType))

	attentionAstString := ast.NewString([]byte(cases.Title(language.English).String(attentionType)))

	// replace the ![TYPE] with a dedicated paragraph of icon+Type
	emphasis.AppendChild(emphasis, attentionAstString)
	attentionParagraph.AppendChild(attentionParagraph, NewAttention(attentionType))
	attentionParagraph.AppendChild(attentionParagraph, emphasis)
	firstParagraph.Parent().InsertBefore(firstParagraph.Parent(), firstParagraph, attentionParagraph)
	firstParagraph.RemoveChild(firstParagraph, node1)
	firstParagraph.RemoveChild(firstParagraph, node2)
	firstParagraph.RemoveChild(firstParagraph, node3)
	if firstParagraph.ChildCount() == 0 {
		firstParagraph.Parent().RemoveChild(firstParagraph.Parent(), firstParagraph)
	}
	return ast.WalkContinue, nil
}
