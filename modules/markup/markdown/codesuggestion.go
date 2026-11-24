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
func isCodeSuggestionInfo(info []byte) bool {
	word, _, _ := bytes.Cut(bytes.TrimSpace(info), []byte(" "))
	word, _, _ = bytes.Cut(word, []byte("\t"))
	return bytes.EqualFold(word, []byte("suggestion"))
}

func codeSuggestionContent(n *ast.FencedCodeBlock, source []byte) string {
	return strings.ReplaceAll(string(n.Lines().Value(source)), "\r\n", "\n")
}

// ParseCodeSuggestions returns the replacement text of every code suggestion block in the markdown content, in document order
func ParseCodeSuggestions(content string) (suggestions []string) {
	if !strings.Contains(strings.ToLower(content), "suggestion") {
		return nil
	}
	source := []byte(content)
	doc := goldmarkDefaultParser().Parse(text.NewReader(source))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if fenced, ok := n.(*ast.FencedCodeBlock); ok && entering && fenced.Info != nil && isCodeSuggestionInfo(fenced.Info.Segment.Value(source)) {
			suggestions = append(suggestions, codeSuggestionContent(fenced, source))
		}
		return ast.WalkContinue, nil
	})
	return suggestions
}

func (r *HTMLRenderer) renderCodeSuggestion(w util.BufWriter, anchor markup.CodeSuggestionAnchor, suggestion markup.CodeSuggestionSource, replacement string) error {
	if err := r.renderInternal.FormatWithSafeAttrsf(w,
		`<div class="code-suggestion"><div class="code-suggestion-heading">%s</div><div class="code-suggestion-diff code-diff-unified"><table class="chroma"><tbody>`,
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
			if err := r.renderInternal.FormatWithSafeAttrsf(w,
				`<tr class="%s"><td class="lines-num">%d</td><td class="lines-type-marker">%s</td><td class="lines-code"><div class="code-inner">%s</div></td></tr>`,
				change.class, anchor.StartLine+uint64(lineIndex), change.marker, line); err != nil {
				return err
			}
		}
	}
	_, err := w.WriteString("</tbody></table></div></div>")
	return err
}
