// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"context"
	"strings"
	"testing"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodeSuggestions(t *testing.T) {
	for _, test := range []struct {
		name, content string
		want          []string
	}{
		{"TildeFenceWithOptionsAndCRLF", "~~~ Suggestion foo=bar\r\nfoo\r\nbar\r\n~~~", []string{"foo\nbar\n"}},
		{"LongFenceWithEmbeddedBackticks", "````suggestion\nfoo ```x```\n```\n````", []string{"foo ```x```\n```\n"}},
		{"EmptyFence", "```suggestion\n```", []string{""}},
		{"NestedSuggestionsInDocumentOrder", "> ```suggestion\n> quoted\n> ```\n\n- item\n\n  ```suggestion\n  listed\n  ```", []string{"quoted\n", "listed\n"}},
		{"UnclosedFence", "```suggestion\nunclosed\n", []string{"unclosed\n"}},
		{"OrdinaryCodeBlocks", "```suggestion:-1+1\nfoo\n```\n\n```go\nsuggestion\n```\n\n    ```suggestion\n    indented code\n    ```", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, ParseCodeSuggestions(test.content))
		})
	}
}

func TestRenderCodeSuggestions(t *testing.T) {
	originalHelpers := markup.DefaultRenderHelperFuncs
	t.Cleanup(func() { markup.DefaultRenderHelperFuncs = originalHelpers })
	markup.DefaultRenderHelperFuncs = &markup.RenderHelperFuncs{
		GetCodeSuggestionSource: func(_ctx context.Context, anchor markup.CodeSuggestionAnchor) (markup.CodeSuggestionSource, error) {
			switch anchor.FilePath {
			case "main.go":
				return markup.CodeSuggestionSource{Heading: "Suggested <change>", OriginalLines: []string{"var previous = 1", "// <tag>"}}, nil
			case "empty.txt":
				return markup.CodeSuggestionSource{OriginalLines: []string{""}}, nil
			default:
				return markup.CodeSuggestionSource{}, util.ErrNotExist
			}
		},
	}

	for _, test := range []struct {
		name, content string
		anchor        *markup.CodeSuggestionAnchor
		adds, dels    int
		fragments     []string
	}{
		{
			"HighlightedRangeInQuote", "> ~~~Suggestion foo=bar\n> var next = 2\n> var extra = 3\n> \n> ~~~",
			&markup.CodeSuggestionAnchor{FilePath: "main.go", StartLine: 12, EndLine: 13}, 3, 2,
			[]string{`Suggested &lt;change&gt;`, `class="lines-num">12</div>`, `class="lines-num">14</div>`, `class="lines-type-marker">-</div>`, `class="lines-type-marker">+</div>`, `<span class="kd">var</span>`, "&lt;tag&gt;", "</blockquote>"},
		},
		{"Deletion", "```suggestion\n```", &markup.CodeSuggestionAnchor{FilePath: "empty.txt", StartLine: 2, EndLine: 2}, 0, 1, []string{`class="lines-num">2</div>`}},
		{"WithoutAnchor", "```suggestion\nplain\n```", nil, 0, 0, []string{"language-suggestion", "plain\n</code></pre>"}},
		{"UnavailableSource", "```suggestion\nplain\n```", &markup.CodeSuggestionAnchor{FilePath: "missing.txt", StartLine: 2, EndLine: 2}, 0, 0, []string{"language-suggestion", "plain\n</code></pre>"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := markup.NewRenderContext(t.Context())
			ctx.RenderOptions.CodeSuggestion = test.anchor
			rendered, err := RenderString(ctx, test.content)
			require.NoError(t, err)
			for _, fragment := range test.fragments {
				assert.Contains(t, string(rendered), fragment)
			}
			assert.Equal(t, test.adds, strings.Count(string(rendered), `class="code-suggestion-row add-code"`))
			assert.Equal(t, test.dels, strings.Count(string(rendered), `class="code-suggestion-row del-code"`))
		})
	}
}
