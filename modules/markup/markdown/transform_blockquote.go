// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"

	"code.gitea.io/gitea/modules/markup/markdown/math"
	"code.gitea.io/gitea/modules/svg"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// renderAttention renders a quote marked with i.e. "> **Note**" or "> **Warning**" with a corresponding svg
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

func popAttentionTypeFromMathBlock(g *ASTTransformer, mathBlock *math.Block, reader text.Reader) string {
	line := mathBlock.Lines().At(0)
	innerText := line.Value(reader.Source())

	// make sure it's a !TYPE
	if innerText[0] != '!' {
		return ""
	}
	attentionType := strings.ToLower(string(innerText[1:]))
	if !g.attentionTypes.Contains(attentionType) {
		return ""
	}
	return attentionType
}

func popAttentionTypeFromParagraph(g *ASTTransformer, paragraph *ast.Paragraph, reader text.Reader) string {
	g.applyElementDir(paragraph)
	if paragraph.ChildCount() < 3 {
		return ""
	}
	node1, ok := paragraph.FirstChild().(*ast.Text)
	if !ok {
		return ""
	}
	node2, ok := node1.NextSibling().(*ast.Text)
	if !ok {
		return ""
	}
	node3, ok := node2.NextSibling().(*ast.Text)
	if !ok {
		return ""
	}
	val1 := string(node1.Segment.Value(reader.Source()))
	val2 := string(node2.Segment.Value(reader.Source()))
	val3 := string(node3.Segment.Value(reader.Source()))
	if val1 != "[" || val3 != "]" || !strings.HasPrefix(val2, "!") {
		return ""
	}

	attentionType := strings.ToLower(val2[1:])
	if !g.attentionTypes.Contains(attentionType) {
		return ""
	}

	paragraph.RemoveChild(paragraph, node1)
	paragraph.RemoveChild(paragraph, node2)
	paragraph.RemoveChild(paragraph, node3)
	return attentionType
}

func newAttentionParagraph(v *ast.Blockquote, attentionType string, g *ASTTransformer) *ast.Paragraph {
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
	return attentionParagraph
}

func (g *ASTTransformer) transformBlockquote(v *ast.Blockquote, reader text.Reader) (ast.WalkStatus, error) {
	// We only want attention blockquotes when the AST looks like:
	// > Text("[") Text("!TYPE") Text("]")
	//
	// or, in case of a math block: \[!TYPE\]

	firstChild := v.FirstChild()
	var attentionType string

	// grab attention type from markdown source
	if paragraph, ok := firstChild.(*ast.Paragraph); ok {
		attentionType = popAttentionTypeFromParagraph(g, paragraph, reader)
	} else {
		mathBlock, ok := firstChild.(*math.Block)
		if !ok {
			return ast.WalkContinue, nil
		}
		attentionType = popAttentionTypeFromMathBlock(g, mathBlock, reader)
	}

	// it's possible this isn't an attention block
	if attentionType == "" {
		return ast.WalkContinue, nil
	}

	attentionParagraph := newAttentionParagraph(v, attentionType, g)
	v.InsertBefore(v, firstChild, attentionParagraph)
	if firstChild.ChildCount() == 0 {
		v.RemoveChild(v, firstChild)
	}
	return ast.WalkContinue, nil
}
