// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"net/url"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/translation"

	"github.com/yuin/goldmark/ast"
)

func createTOCNode(toc []markup.Header, lang string, detailsAttrs map[string]string) ast.Node {
	details := NewDetails()
	summary := NewSummary()

	for k, v := range detailsAttrs {
		details.SetAttributeString(k, []byte(v))
	}

	summary.AppendChild(summary, ast.NewString([]byte(translation.NewLocale(lang).Tr("toc"))))
	details.AppendChild(details, summary)
	ul := ast.NewList('-')
	details.AppendChild(details, ul)
	currentLevel := 6
	for _, header := range toc {
		if header.Level < currentLevel {
			currentLevel = header.Level
		}
	}
	for _, header := range toc {
		for currentLevel > header.Level {
			ul = ul.Parent().(*ast.List)
			currentLevel--
		}
		for currentLevel < header.Level {
			newL := ast.NewList('-')
			ul.AppendChild(ul, newL)
			currentLevel++
			ul = newL
		}
		li := ast.NewListItem(currentLevel * 2)
		a := ast.NewLink()
		a.Destination = []byte(fmt.Sprintf("#%s", url.QueryEscape(header.ID)))
		a.AppendChild(a, ast.NewString([]byte(header.Text)))
		li.AppendChild(li, a)
		ul.AppendChild(ul, li)
	}

	return details
}
