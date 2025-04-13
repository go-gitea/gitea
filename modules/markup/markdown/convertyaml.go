// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"

	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/svg"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"gopkg.in/yaml.v3"
)

func nodeToTable(meta *yaml.Node) ast.Node {
	for meta != nil && meta.Kind == yaml.DocumentNode {
		meta = meta.Content[0]
	}
	if meta == nil {
		return nil
	}
	switch meta.Kind {
	case yaml.MappingNode:
		return mappingNodeToTable(meta)
	case yaml.SequenceNode:
		return sequenceNodeToTable(meta)
	default:
		return ast.NewString([]byte(meta.Value))
	}
}

func mappingNodeToTable(meta *yaml.Node) ast.Node {
	table := east.NewTable()
	alignments := make([]east.Alignment, 0, len(meta.Content)/2)
	for i := 0; i < len(meta.Content); i += 2 {
		alignments = append(alignments, east.AlignNone)
	}

	headerRow := east.NewTableRow(alignments)
	valueRow := east.NewTableRow(alignments)
	for i := 0; i < len(meta.Content); i += 2 {
		cell := east.NewTableCell()

		cell.AppendChild(cell, nodeToTable(meta.Content[i]))
		headerRow.AppendChild(headerRow, cell)

		if i+1 < len(meta.Content) {
			cell = east.NewTableCell()
			cell.AppendChild(cell, nodeToTable(meta.Content[i+1]))
			valueRow.AppendChild(valueRow, cell)
		}
	}

	table.AppendChild(table, east.NewTableHeader(headerRow))
	table.AppendChild(table, valueRow)
	return table
}

func sequenceNodeToTable(meta *yaml.Node) ast.Node {
	table := east.NewTable()
	alignments := []east.Alignment{east.AlignNone}
	for _, item := range meta.Content {
		row := east.NewTableRow(alignments)
		cell := east.NewTableCell()
		cell.AppendChild(cell, nodeToTable(item))
		row.AppendChild(row, cell)
		table.AppendChild(table, row)
	}
	return table
}

func nodeToDetails(g *ASTTransformer, meta *yaml.Node) ast.Node {
	for meta != nil && meta.Kind == yaml.DocumentNode {
		meta = meta.Content[0]
	}
	if meta == nil {
		return nil
	}
	if meta.Kind != yaml.MappingNode {
		return nil
	}
	var keys []string
	for i := 0; i < len(meta.Content); i += 2 {
		if meta.Content[i].Kind == yaml.ScalarNode {
			keys = append(keys, meta.Content[i].Value)
		}
	}
	details := NewDetails()
	details.SetAttributeString(g.renderInternal.SafeAttr("class"), g.renderInternal.SafeValue("frontmatter-content"))
	summary := NewSummary()
	summaryInnerHTML := htmlutil.HTMLFormat("%s %s", svg.RenderHTML("octicon-table", 12), strings.Join(keys, ", "))
	summary.AppendChild(summary, NewRawHTML(summaryInnerHTML))
	details.AppendChild(details, summary)
	details.AppendChild(details, nodeToTable(meta))
	return details
}
