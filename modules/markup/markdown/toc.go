// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"net/url"

	"code.gitea.io/gitea/modules/translation/i18n"

	"github.com/yuin/goldmark/ast"
)

func createTOCNode(toc []Header, lang string) ast.Node {
	details := NewDetails()
	summary := NewSummary()

	summary.AppendChild(summary, ast.NewString([]byte(i18n.Tr(lang, "toc"))))
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
		a.Destination = []byte(fmt.Sprintf("#%s", url.PathEscape(header.ID)))
		a.AppendChild(a, ast.NewString([]byte(header.Text)))
		li.AppendChild(li, a)
		ul.AppendChild(ul, li)
	}

	return details
}
