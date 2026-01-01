// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"html"
	"html/template"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/translation"
)

// RenderSidebarTocHTML renders a list of headers into HTML for sidebar TOC display.
// It generates a <details> element with nested <ul> lists representing the header hierarchy.
func RenderSidebarTocHTML(headers []Header, lang string) template.HTML {
	if len(headers) == 0 {
		return ""
	}

	var sb strings.Builder

	// Start with <details open>
	sb.WriteString(`<details open>`)
	sb.WriteString(`<summary>`)
	sb.WriteString(html.EscapeString(translation.NewLocale(lang).TrString("toc")))
	sb.WriteString(`</summary>`)

	// Find the minimum level to start with
	minLevel := 6
	for _, header := range headers {
		if header.Level < minLevel {
			minLevel = header.Level
		}
	}

	// Build nested list structure
	currentLevel := minLevel
	sb.WriteString(`<ul>`)
	openLists := 1

	for _, header := range headers {
		// Close lists if we need to go up levels
		for currentLevel > header.Level {
			sb.WriteString(`</ul>`)
			openLists--
			currentLevel--
		}

		// Open new lists if we need to go down levels
		for currentLevel < header.Level {
			sb.WriteString(`<ul>`)
			openLists++
			currentLevel++
		}

		// Write the list item with link
		sb.WriteString(`<li>`)
		sb.WriteString(`<a href="#`)
		sb.WriteString(url.QueryEscape(header.ID))
		sb.WriteString(`">`)
		sb.WriteString(html.EscapeString(header.Text))
		sb.WriteString(`</a>`)
		sb.WriteString(`</li>`)
	}

	// Close all remaining open lists
	for openLists > 0 {
		sb.WriteString(`</ul>`)
		openLists--
	}

	sb.WriteString(`</details>`)

	return template.HTML(sb.String())
}

