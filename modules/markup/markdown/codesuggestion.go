// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"html/template"
	"strings"

	"gitea.dev/modules/highlight"
	"gitea.dev/modules/markup"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const codeSuggestionAttribute = "gitea-code-suggestion"

// GitHub treats "suggestion:-1+2" as a plain fence
func isCodeSuggestionFence(fenced *ast.FencedCodeBlock, source []byte) bool {
	if fenced.Info == nil {
		return false
	}
	word, _, _ := bytes.Cut(bytes.TrimSpace(fenced.Info.Segment.Value(source)), []byte(" "))
	word, _, _ = bytes.Cut(word, []byte("\t"))
	return bytes.EqualFold(word, []byte("suggestion"))
}

// ParseCodeSuggestions returns the replacement text of every code suggestion block in the markdown content, in document order
func ParseCodeSuggestions(content string) (suggestions []string) {
	if !strings.Contains(strings.ToLower(content), "suggestion") {
		return nil
	}
	source := []byte(content)
	doc := goldmarkDefaultParser().Parse(text.NewReader(source))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if fenced, ok := n.(*ast.FencedCodeBlock); ok && entering && isCodeSuggestionFence(fenced, source) {
			suggestions = append(suggestions, strings.ReplaceAll(string(fenced.Lines().Value(source)), "\r\n", "\n"))
		}
		return ast.WalkContinue, nil
	})
	return suggestions
}

func (r *HTMLRenderer) renderCodeSuggestion(w util.BufWriter, anchor markup.CodeSuggestionAnchor, suggestion markup.CodeSuggestionSource, replacement string) error {
	if err := r.internalVar.Value.FormatWithSafeAttrsf(w,
		`<div class="code-suggestion"><div class="code-suggestion-heading">%s</div><div class="code-suggestion-diff"><div class="code-suggestion-lines chroma code-diff-unified">`,
		suggestion.Heading); err != nil {
		return err
	}
	lexer := highlight.DetectChromaLexerByFileName(anchor.FilePath, "")
	highlightLines := func(code string) (lines []template.HTML) {
		if code != "" {
			for _, line := range highlight.UnsafeSplitHighlightedLines(highlight.RenderCodeByLexer(lexer, code)) {
				lines = append(lines, template.HTML(line))
			}
		}
		return lines
	}
	deletions, additions := highlightLines(strings.Join(suggestion.OriginalLines, "\n")+"\n"), highlightLines(replacement)
	if suggestion.HighlightLinePair != nil {
		for i := range min(len(deletions), len(additions)) {
			deletions[i], additions[i] = suggestion.HighlightLinePair(deletions[i], additions[i])
		}
	}
	for _, change := range []struct {
		class, marker string
		lines         []template.HTML
	}{{"del-code", "-", deletions}, {"add-code", "+", additions}} {
		for lineIndex, line := range change.lines {
			if err := r.internalVar.Value.FormatWithSafeAttrsf(w,
				`<div class="code-suggestion-row %s"><div class="code-suggestion-cell lines-num">%d</div><div class="code-suggestion-cell lines-type-marker">%s</div><div class="code-suggestion-cell lines-code"><code class="code-inner">%s</code></div></div>`,
				change.class, anchor.StartLine+uint64(lineIndex), change.marker, line); err != nil {
				return err
			}
		}
	}
	_, err := w.WriteString("</div></div></div>")
	return err
}
