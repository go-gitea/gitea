// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
)

func testInput() string {
	s := `  space @mention-user<SPACE><SPACE>
/just/a/path.bin
https://example.com/file.bin
[local link](file.bin)
[remote link](https://example.com)
[[local link|file.bin]]
[[remote link|https://example.com]]
![local image](image.jpg)
![remote image](https://example.com/image.jpg)
[[local image|image.jpg]]
[[remote link|https://example.com/image.jpg]]
https://example.com/user/repo/compare/88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb#hash
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb pare
https://example.com/user/repo/commit/88fc37a3c0a4dda553bdcfc80c178a58247f42fb
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb mit
:+1:
mail@domain.com
@mention-user test
#123
  space<SPACE><SPACE>
`
	return strings.ReplaceAll(s, "<SPACE>", " ")
}

func TestMain(m *testing.M) {
	setting.Markdown.RenderOptionsComment.ShortIssuePattern = true
	markup.Init(&markup.RenderHelperFuncs{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == "mention-user"
		},
	})
	os.Exit(m.Run())
}

func newTestRenderUtils(t *testing.T) *RenderUtils {
	ctx := reqctx.NewRequestContextForTest(t.Context())
	ctx.SetContextValue(translation.ContextKey, &translation.MockLocale{})
	return NewRenderUtils(ctx)
}

func TestRenderRepoComment(t *testing.T) {
	mockRepo := &repo.Repository{
		ID: 1, OwnerName: "user13", Name: "repo11",
		Owner: &user_model.User{ID: 13, Name: "user13"},
		Units: []*repo.RepoUnit{},
	}
	t.Run("RenderCommitBody", func(t *testing.T) {
		defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
		type args struct {
			msg string
		}
		tests := []struct {
			name string
			args args
			want template.HTML
		}{
			{
				name: "multiple lines",
				args: args{
					msg: "first line\nsecond line",
				},
				want: "second line",
			},
			{
				name: "multiple lines with leading newlines",
				args: args{
					msg: "\n\n\n\nfirst line\nsecond line",
				},
				want: "second line",
			},
			{
				name: "multiple lines with trailing newlines",
				args: args{
					msg: "first line\nsecond line\n\n\n",
				},
				want: "second line",
			},
		}
		ut := newTestRenderUtils(t)
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equalf(t, tt.want, ut.RenderCommitBody(tt.args.msg, mockRepo), "RenderCommitBody(%v, %v)", tt.args.msg, nil)
			})
		}

		expected := `/just/a/path.bin
<a href="https://example.com/file.bin">https://example.com/file.bin</a>
[local link](file.bin)
[remote link](<a href="https://example.com">https://example.com</a>)
[[local link|file.bin]]
[[remote link|<a href="https://example.com">https://example.com</a>]]
![local image](image.jpg)
![remote image](<a href="https://example.com/image.jpg">https://example.com/image.jpg</a>)
[[local image|image.jpg]]
[[remote link|<a href="https://example.com/image.jpg">https://example.com/image.jpg</a>]]
<a href="https://example.com/user/repo/compare/88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb#hash" class="compare"><code>88fc37a3c0...12fc37a3c0 (hash)</code></a>
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb pare
<a href="https://example.com/user/repo/commit/88fc37a3c0a4dda553bdcfc80c178a58247f42fb" class="commit"><code>88fc37a3c0</code></a>
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb mit
<span class="emoji" aria-label="thumbs up">👍</span>
<a href="mailto:mail@domain.com">mail@domain.com</a>
<a href="/mention-user">@mention-user</a> test
<a href="/user13/repo11/issues/123" class="ref-issue">#123</a>
  space`
		assert.Equal(t, expected, string(newTestRenderUtils(t).RenderCommitBody(testInput(), mockRepo)))
	})

	t.Run("RenderCommitMessage", func(t *testing.T) {
		expected := `space <a href="/mention-user" data-markdown-generated-content="">@mention-user</a>  `
		assert.EqualValues(t, expected, newTestRenderUtils(t).RenderCommitMessage(testInput(), mockRepo))
	})

	t.Run("RenderCommitMessageLinkSubject", func(t *testing.T) {
		expected := `<a href="https://example.com/link" class="muted">space </a><a href="/mention-user" data-markdown-generated-content="">@mention-user</a>`
		assert.EqualValues(t, expected, newTestRenderUtils(t).RenderCommitMessageLinkSubject(testInput(), "https://example.com/link", mockRepo))
	})

	t.Run("RenderIssueTitle", func(t *testing.T) {
		defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
		expected := `  space @mention-user<SPACE><SPACE>
/just/a/path.bin
https://example.com/file.bin
[local link](file.bin)
[remote link](https://example.com)
[[local link|file.bin]]
[[remote link|https://example.com]]
![local image](image.jpg)
![remote image](https://example.com/image.jpg)
[[local image|image.jpg]]
[[remote link|https://example.com/image.jpg]]
https://example.com/user/repo/compare/88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb#hash
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb pare
https://example.com/user/repo/commit/88fc37a3c0a4dda553bdcfc80c178a58247f42fb
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb mit
<span class="emoji" aria-label="thumbs up">👍</span>
mail@domain.com
@mention-user test
<a href="/user13/repo11/issues/123" class="ref-issue">#123</a>
  space<SPACE><SPACE>
`
		expected = strings.ReplaceAll(expected, "<SPACE>", " ")
		assert.Equal(t, expected, string(newTestRenderUtils(t).RenderIssueTitle(testInput(), mockRepo)))
	})
}

func TestRenderMarkdownToHtml(t *testing.T) {
	defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	expected := `<p>space <a href="/mention-user" rel="nofollow">@mention-user</a><br/>
/just/a/path.bin
<a href="https://example.com/file.bin" rel="nofollow">https://example.com/file.bin</a>
<a href="/file.bin" rel="nofollow">local link</a>
<a href="https://example.com" rel="nofollow">remote link</a>
<a href="/file.bin" rel="nofollow">local link</a>
<a href="https://example.com" rel="nofollow">remote link</a>
<a href="/image.jpg" target="_blank" rel="nofollow noopener"><img src="/image.jpg" alt="local image"/></a>
<a href="https://example.com/image.jpg" target="_blank" rel="nofollow noopener"><img src="https://example.com/image.jpg" alt="remote image"/></a>
<a href="/image.jpg" rel="nofollow"><img src="/image.jpg" title="local image" alt="local image"/></a>
<a href="https://example.com/image.jpg" rel="nofollow"><img src="https://example.com/image.jpg" title="remote link" alt="remote link"/></a>
<a href="https://example.com/user/repo/compare/88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb#hash" rel="nofollow"><code>88fc37a3c0...12fc37a3c0 (hash)</code></a>
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb pare
<a href="https://example.com/user/repo/commit/88fc37a3c0a4dda553bdcfc80c178a58247f42fb" rel="nofollow"><code>88fc37a3c0</code></a>
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb mit
<span class="emoji" aria-label="thumbs up">👍</span>
<a href="mailto:mail@domain.com" rel="nofollow">mail@domain.com</a>
<a href="/mention-user" rel="nofollow">@mention-user</a> test
#123
space</p>
`
	assert.Equal(t, expected, string(newTestRenderUtils(t).MarkdownToHtml(testInput())))
}

func TestRenderLabels(t *testing.T) {
	ut := newTestRenderUtils(t)
	label := &issues.Label{ID: 123, Name: "label-name", Color: "label-color"}
	issue := &issues.Issue{}
	expected := `/owner/repo/issues?labels=123`
	assert.Contains(t, ut.RenderLabels([]*issues.Label{label}, "/owner/repo", issue), expected)

	label = &issues.Label{ID: 123, Name: "label-name", Color: "label-color"}
	issue = &issues.Issue{IsPull: true}
	expected = `/owner/repo/pulls?labels=123`
	assert.Contains(t, ut.RenderLabels([]*issues.Label{label}, "/owner/repo", issue), expected)
}

func TestUserMention(t *testing.T) {
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	rendered := newTestRenderUtils(t).MarkdownToHtml("@no-such-user @mention-user @mention-user")
	assert.Equal(t, `<p>@no-such-user <a href="/mention-user" rel="nofollow">@mention-user</a> <a href="/mention-user" rel="nofollow">@mention-user</a></p>`, strings.TrimSpace(string(rendered)))
}
