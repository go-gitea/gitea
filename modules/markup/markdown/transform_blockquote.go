// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"

	"code.gitea.io/gitea/modules/svg"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// renderAttention renders a quote marked with i.e. "> **Note**" or "> [!Warning]" with a corresponding svg
func (r *HTMLRenderer) renderAttention(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*Attention)
		var octiconName string
		switch n.AttentionType {
		case "tip":
			octiconName = "light-bulb"
		case "important":
			octiconName = "report"
		case "warning":
			octiconName = "alert"
		case "caution":
			octiconName = "stop"
		default: // including "note"
			octiconName = "info"
		}
		_, _ = w.WriteString(string(svg.RenderHTML("octicon-"+octiconName, 16, "attention-icon attention-"+n.AttentionType)))
	}
	return ast.WalkContinue, nil
}

func (g *ASTTransformer) extractBlockquoteAttentionEmphasis(firstParagraph ast.Node, reader text.Reader) (string, []ast.Node) {
	if firstParagraph.ChildCount() < 1 {
		return "", nil
	}
	node1, ok := firstParagraph.FirstChild().(*ast.Emphasis)
	if !ok {
		return "", nil
	}
	val1 := string(node1.Text(reader.Source()))
	attentionType := strings.ToLower(val1)
	if g.attentionTypes.Contains(attentionType) {
		return attentionType, []ast.Node{node1}
	}
	return "", nil
}

func (g *ASTTransformer) extractBlockquoteAttention2(firstParagraph ast.Node, reader text.Reader) (string, []ast.Node) {
	if firstParagraph.ChildCount() < 2 {
		return "", nil
	}
	node1, ok := firstParagraph.FirstChild().(*ast.Text)
	if !ok {
		return "", nil
	}
	node2, ok := node1.NextSibling().(*ast.Text)
	if !ok {
		return "", nil
	}
	val1 := string(node1.Segment.Value(reader.Source()))
	val2 := string(node2.Segment.Value(reader.Source()))
	if strings.HasPrefix(val1, `\[!`) && val2 == `\]` {
		attentionType := strings.ToLower(val1[3:])
		if g.attentionTypes.Contains(attentionType) {
			return attentionType, []ast.Node{node1, node2}
		}
	}
	return "", nil
}

func (g *ASTTransformer) extractBlockquoteAttention3(firstParagraph ast.Node, reader text.Reader) (string, []ast.Node) {
	if firstParagraph.ChildCount() < 3 {
		return "", nil
	}
	node1, ok := firstParagraph.FirstChild().(*ast.Text)
	if !ok {
		return "", nil
	}
	node2, ok := node1.NextSibling().(*ast.Text)
	if !ok {
		return "", nil
	}
	node3, ok := node2.NextSibling().(*ast.Text)
	if !ok {
		return "", nil
	}
	val1 := string(node1.Segment.Value(reader.Source()))
	val2 := string(node2.Segment.Value(reader.Source()))
	val3 := string(node3.Segment.Value(reader.Source()))
	if val1 != "[" || val3 != "]" || !strings.HasPrefix(val2, "!") {
		return "", nil
	}

	attentionType := strings.ToLower(val2[1:])
	if g.attentionTypes.Contains(attentionType) {
		return attentionType, []ast.Node{node1, node2, node3}
	}
	return "", nil
}

func (g *ASTTransformer) transformBlockquote(v *ast.Blockquote, reader text.Reader) (ast.WalkStatus, error) {
	// We only want attention blockquotes when the AST looks like:
	// > Text("[") Text("!TYPE") Text("]")
	// > Text("\[!TYPE") TEXT("\]")
	// > Text("**TYPE**")

	// grab these nodes and make sure we adhere to the attention blockquote structure
	firstParagraph := v.FirstChild()
	g.applyElementDir(firstParagraph)

	attentionType, processedNodes := g.extractBlockquoteAttentionEmphasis(firstParagraph, reader)
	if attentionType == "" {
		attentionType, processedNodes = g.extractBlockquoteAttention2(firstParagraph, reader)
	}
	if attentionType == "" {
		attentionType, processedNodes = g.extractBlockquoteAttention3(firstParagraph, reader)
	}
	if attentionType == "" {
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
	for _, processed := range processedNodes {
		firstParagraph.RemoveChild(firstParagraph, processed)
	}
	if firstParagraph.ChildCount() == 0 {
		firstParagraph.Parent().RemoveChild(firstParagraph.Parent(), firstParagraph)
	}
	return ast.WalkContinue, nil
}
