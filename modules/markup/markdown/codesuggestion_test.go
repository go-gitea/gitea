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
	for _, test := range []struct {
		name, content string
		anchor        *markup.CodeSuggestionAnchor
		original      []string
		adds, dels    int
		fragments     []string
	}{
		{
			"HighlightedRangeInQuote", "Before\n\n> ~~~Suggestion foo=bar\n> var next = 2\n> var extra = 3\n> \n> ~~~\n\nAfter",
			&markup.CodeSuggestionAnchor{FilePath: "main.go", StartLine: 12, EndLine: 13},
			[]string{"var previous = 1", "// <tag>"},
			3, 2,
			[]string{
				`class="code-suggestion-heading">Suggested &lt;change&gt;`, `class="lines-num">12</td>`, `class="lines-num">13</td>`,
				`class="lines-num">14</td>`, `<span class="kd">var</span>`, "&lt;tag&gt;", "Before</p>", "</blockquote>", "After</p>",
			},
		},
		{
			"Deletion", "```suggestion\n```", &markup.CodeSuggestionAnchor{FilePath: "file.txt", StartLine: 2, EndLine: 2},
			[]string{""},
			0, 1,
			[]string{`class="lines-num">2</td>`},
		},
		{"WithoutAnchor", "```suggestion\nplain\n```", nil, nil, 0, 0, []string{"language-suggestion", "plain\n</code></pre>"}},
		{
			"UnavailableSource", "```suggestion\nplain\n```", &markup.CodeSuggestionAnchor{FilePath: "file.txt", StartLine: 2, EndLine: 2}, nil,
			0, 0,
			[]string{"language-suggestion", "plain\n</code></pre>"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalHelpers := markup.DefaultRenderHelperFuncs
			t.Cleanup(func() { markup.DefaultRenderHelperFuncs = originalHelpers })
			markup.DefaultRenderHelperFuncs = &markup.RenderHelperFuncs{
				GetCodeSuggestionSource: func(_ctx context.Context, anchor markup.CodeSuggestionAnchor) (markup.CodeSuggestionSource, error) {
					require.NotNil(t, test.anchor)
					assert.Equal(t, *test.anchor, anchor)
					if test.original == nil {
						return markup.CodeSuggestionSource{}, util.ErrNotExist
					}
					return markup.CodeSuggestionSource{Heading: "Suggested <change>", OriginalLines: test.original}, nil
				},
			}
			ctx := markup.NewRenderContext(t.Context())
			ctx.RenderOptions.CodeSuggestion = test.anchor
			rendered, err := RenderString(ctx, test.content)
			require.NoError(t, err)
			for _, fragment := range test.fragments {
				assert.Contains(t, string(rendered), fragment)
			}
			assert.Equal(t, test.adds, strings.Count(string(rendered), `class="add-code"`))
			assert.Equal(t, test.dels, strings.Count(string(rendered), `class="del-code"`))
		})
	}
}
