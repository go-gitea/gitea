// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"gopkg.in/yaml.v2"
)

// RenderConfig represents rendering configuration for this file
type RenderConfig struct {
	Meta string
	Icon string
	TOC  bool
	Lang string
}

// ToRenderConfig converts a yaml.MapSlice to a RenderConfig
func (rc *RenderConfig) ToRenderConfig(meta yaml.MapSlice) {
	if meta == nil {
		return
	}
	found := false
	var giteaMetaControl yaml.MapItem
	for _, item := range meta {
		strKey, ok := item.Key.(string)
		if !ok {
			continue
		}
		strKey = strings.TrimSpace(strings.ToLower(strKey))
		switch strKey {
		case "gitea":
			giteaMetaControl = item
			found = true
		case "include_toc":
			val, ok := item.Value.(bool)
			if !ok {
				continue
			}
			rc.TOC = val
		case "lang":
			val, ok := item.Value.(string)
			if !ok {
				continue
			}
			val = strings.TrimSpace(val)
			if len(val) == 0 {
				continue
			}
			rc.Lang = val
		}
	}

	if found {
		switch v := giteaMetaControl.Value.(type) {
		case string:
			switch v {
			case "none":
				rc.Meta = "none"
			case "table":
				rc.Meta = "table"
			default: // "details"
				rc.Meta = "details"
			}
		case yaml.MapSlice:
			for _, item := range v {
				strKey, ok := item.Key.(string)
				if !ok {
					continue
				}
				strKey = strings.TrimSpace(strings.ToLower(strKey))
				switch strKey {
				case "meta":
					val, ok := item.Value.(string)
					if !ok {
						continue
					}
					switch strings.TrimSpace(strings.ToLower(val)) {
					case "none":
						rc.Meta = "none"
					case "table":
						rc.Meta = "table"
					default: // "details"
						rc.Meta = "details"
					}
				case "details_icon":
					val, ok := item.Value.(string)
					if !ok {
						continue
					}
					rc.Icon = strings.TrimSpace(strings.ToLower(val))
				case "include_toc":
					val, ok := item.Value.(bool)
					if !ok {
						continue
					}
					rc.TOC = val
				case "lang":
					val, ok := item.Value.(string)
					if !ok {
						continue
					}
					val = strings.TrimSpace(val)
					if len(val) == 0 {
						continue
					}
					rc.Lang = val
				}
			}
		}
	}
}

func (rc *RenderConfig) toMetaNode(meta yaml.MapSlice) ast.Node {
	switch rc.Meta {
	case "table":
		return metaToTable(meta)
	case "details":
		return metaToDetails(meta, rc.Icon)
	default:
		return nil
	}
}

func metaToTable(meta yaml.MapSlice) ast.Node {
	table := east.NewTable()
	alignments := []east.Alignment{}
	for range meta {
		alignments = append(alignments, east.AlignNone)
	}
	row := east.NewTableRow(alignments)
	for _, item := range meta {
		cell := east.NewTableCell()
		cell.AppendChild(cell, ast.NewString([]byte(fmt.Sprintf("%v", item.Key))))
		row.AppendChild(row, cell)
	}
	table.AppendChild(table, east.NewTableHeader(row))

	row = east.NewTableRow(alignments)
	for _, item := range meta {
		cell := east.NewTableCell()
		cell.AppendChild(cell, ast.NewString([]byte(fmt.Sprintf("%v", item.Value))))
		row.AppendChild(row, cell)
	}
	table.AppendChild(table, row)
	return table
}

func metaToDetails(meta yaml.MapSlice, icon string) ast.Node {
	details := NewDetails()
	summary := NewSummary()
	summary.AppendChild(summary, NewIcon(icon))
	details.AppendChild(details, summary)
	details.AppendChild(details, metaToTable(meta))

	return details
}
