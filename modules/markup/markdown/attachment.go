// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"

	"code.gitea.io/gitea/modules/container"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

func ParseAttachments(content string) (container.Set[string], error) {
	parser := goldmark.DefaultParser()
	rd := text.NewReader([]byte(content))
	doc := parser.Parse(rd)
	var attachments []string
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch b := n.(type) {
		case *ast.HTMLBlock:
			var tag string
			for i := 0; i < b.Lines().Len(); i++ {
				p := content[b.Lines().At(i).Start:b.Lines().At(i).Stop]
				tag += p
			}
			doc, err := html.Parse(strings.NewReader(strings.TrimSpace(tag)))
			if err != nil {
				return ast.WalkStop, err
			}

			var processAllImgs func(*html.Node)
			processAllImgs = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "img" {
					for _, a := range n.Attr {
						if a.Key == "src" && strings.HasPrefix(a.Val, "/attachments/") {
							attachments = append(attachments, strings.TrimPrefix(a.Val, "/attachments/"))
							return
						}
					}
				}
				// traverse the child nodes
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					processAllImgs(c)
				}
			}
			// make a recursive call to your function
			processAllImgs(doc)
		case *ast.Image:
			if strings.HasPrefix(string(b.Destination), "/attachments/") {
				attachments = append(attachments, strings.TrimPrefix(string(b.Destination), "/attachments/"))
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return nil, err
	}
	return container.SetOf(attachments...), nil
}
