// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package highlight prepares code for the highlighter of the frontend, which needs the language and the plain text
package highlight

import (
	"html/template"
	"path"
	"strings"
	"sync"

	"gitea.dev/modules/analyze"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

var highlightMapping = sync.OnceValue(setting.GetHighlightMapping)

// DetectLanguage returns the linguist name of a file's language, empty if it is unknown.
// The "highlight.mapping" setting wins over the language of the git attributes, the content is only used if the file name doesn't tell.
func DetectLanguage(fileName, attrLanguage string, content []byte) string {
	if mapped, ok := highlightMapping()[path.Ext(fileName)]; ok {
		if !strings.HasPrefix(mapped, ".") {
			return mapped
		}
		fileName, attrLanguage = "dummy"+mapped, "" // mapped to another extension
	}
	if attrLanguage != "" {
		attrLanguage, _, _ = strings.Cut(attrLanguage, "?") // the attribute may have parameters
		return attrLanguage
	}
	return analyze.GetCodeLanguage(fileName, content)
}

// RenderPlainLines returns the escaped lines of the code, each keeps its "\n" so that copied code keeps its lines
func RenderPlainLines(code string) []template.HTML {
	lines := make([]template.HTML, 0, strings.Count(code, "\n")+1)
	for line := range strings.Lines(code) {
		lines = append(lines, htmlutil.EscapeString(line))
	}
	return lines
}

// CodeBlockAttributes returns the attributes of the "pre" and "code" elements of a code block
func CodeBlockAttributes(lang string) (preAttrs, codeAttrs template.HTML) {
	// "language-{LanguageName}" class is used as part of commonmark spec.
	// It's unclear about how to handle special chars for a language name like "Visual Basic.NET" or "C++" or "F#".
	// The commonmark spec seems wrong: https://spec.commonmark.org/0.31.2/#info-string, it just outputs invalid CSS class names.
	lang = util.IfZero(lang, "text")
	cssName := strings.Map(func(r rune) rune {
		// although "-" is valid in CSS name, it is used as a field separator, so we don't want to keep it in the name
		if 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, strings.ToLower(lang))
	preExtraClasses := ""
	if lang == "mermaid" || lang == "math" { // rendered by the frontend
		preExtraClasses = " is-loading"
	}

	// The "math.ts" strictly depends on the structure: <pre class="code-block"><code class="language-math">...</code></pre>
	// * If "pre" exists, it is rendered as "block", otherwise, it is rendered as "inline"
	// The "mermaid.ts" also strictly depends on the structure: "pre" must exist because it is always rendered as "block".
	//
	// Hint: "data-code-language" is not exposed in some cases due to the Markup sanitizer, the rules can be refactored in the future if the attribute is useful.
	return htmlutil.HTMLFormat(`class="code-block%s"`, preExtraClasses), htmlutil.HTMLFormat(`class="code-highlight language-%s" data-code-language="%s"`, cssName, lang)
}
