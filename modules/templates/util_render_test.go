// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"os"
	"strings"
	"testing"

	"gitea.dev/models/issues"
	"gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/translation"

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

	t.Run("RenderCommitMessageLinkSubjectURLOnly", func(t *testing.T) {
		// a bare URL in the subject must not hijack the default link
		expected := `<a href="https://example.com/link" class="muted">https://example.com/file.bin</a>`
		assert.EqualValues(t, expected, newTestRenderUtils(t).RenderCommitMessageLinkSubject("https://example.com/file.bin", "https://example.com/link", mockRepo))
	})

	t.Run("RenderCommitMessageLinkSubjectPartialURL", func(t *testing.T) {
		// a URL embedded in larger subject text still becomes its own link
		expected := `<a href="https://example.com/link" class="muted">see </a><a href="https://example.com/x" data-markdown-generated-content="">https://example.com/x</a><a href="https://example.com/link" class="muted"> here</a>`
		assert.EqualValues(t, expected, newTestRenderUtils(t).RenderCommitMessageLinkSubject("see https://example.com/x here", "https://example.com/link", mockRepo))
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

func TestRenderIssueTitleCodeSpan(t *testing.T) {
	defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	mockRepo := &repo.Repository{
		ID: 1, OwnerName: "user13", Name: "repo11",
		Owner: &user_model.User{ID: 13, Name: "user13"},
		Units: []*repo.RepoUnit{},
	}
	ut := newTestRenderUtils(t)

	cases := []struct {
		input     string
		expected  string
		emojiSafe bool
	}{
		{"foo `:100:`", `foo <code class="inline-code-block">:100:</code>`, true},
		{"`#123`", `<code class="inline-code-block">#123</code>`, false},
		{"`88fc37a3c0a4dda553bdcfc80c178a58247f42fb`", `<code class="inline-code-block">88fc37a3c0a4dda553bdcfc80c178a58247f42fb</code>`, false},
		{"foo `:100:", "foo `:100:", true},
		{"foo ` :100:", `foo ` + "`" + ` <span class="emoji" aria-label="hundred points">💯</span>`, true},
		{":100:", `<span class="emoji" aria-label="hundred points">💯</span>`, true},
		{"#123", `<a href="/user13/repo11/issues/123" class="ref-issue">#123</a>`, false},
		{"`x`:100:", `<code class="inline-code-block">x</code><span class="emoji" aria-label="hundred points">💯</span>`, true},
		{"a `:100:` b `:+1:` c", `a <code class="inline-code-block">:100:</code> b <code class="inline-code-block">:+1:</code> c`, true},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, string(ut.RenderIssueTitle(c.input, mockRepo)), "input=%q", c.input)
		if c.emojiSafe {
			assert.Equal(t, c.expected, string(ut.RenderIssueSimpleTitle(c.input)), "simple input=%q", c.input)
		}
	}
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

func TestRenderPackageMarkdown(t *testing.T) {
	defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	mockRepo := &repo.Repository{
		ID: 1, OwnerName: "user13", Name: "repo11", DefaultBranch: "main",
		Owner: &user_model.User{ID: 13, Name: "user13"},
		Units: []*repo.RepoUnit{},
	}
	ut := newTestRenderUtils(t)

	t.Run("LinkedRepoWithDirectory", func(t *testing.T) {
		rendered := ut.RenderPackageMarkdown("[docs](docs/getting-started.md)\n![logo](logo.png)", mockRepo, "pkg-subdir")
		expected := `<div class="markup markdown"><p><a href="/user13/repo11/src/branch/main/pkg-subdir/docs/getting-started.md" rel="nofollow">docs</a>
<a href="/user13/repo11/src/branch/main/pkg-subdir/logo.png" target="_blank" rel="nofollow noopener"><img src="/user13/repo11/media/branch/main/pkg-subdir/logo.png" alt="logo"/></a></p>
</div>`
		assert.Equal(t, expected, strings.TrimSpace(string(rendered)))
	})

	t.Run("LinkedRepoWithEmptyDirectory", func(t *testing.T) {
		rendered := ut.RenderPackageMarkdown("[docs](docs/getting-started.md)", mockRepo, "")
		expected := `<div class="markup markdown"><p><a href="/user13/repo11/src/branch/main/docs/getting-started.md" rel="nofollow">docs</a></p>
</div>`
		assert.Equal(t, expected, strings.TrimSpace(string(rendered)))
	})

	t.Run("UnlinkedRepo", func(t *testing.T) {
		rendered := ut.RenderPackageMarkdown("[docs](docs/getting-started.md)", nil, "pkg-subdir")
		expected := `<div class="markup markdown"><p><a href="/docs/getting-started.md" rel="nofollow">docs</a></p>
</div>`
		assert.Equal(t, expected, strings.TrimSpace(string(rendered)))
	})
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

	expectedLabel := `<span class="ui label " style="color: #fff !important; background-color: label-color !important;" data-tooltip-content title=""><span class="gt-ellipsis">label-name</span></span>`
	assert.Equal(t, expectedLabel, string(ut.RenderLabel(label)))

	label = &issues.Label{ID: 123, Name: "</>", Exclusive: true}
	expectedLabel = `<span class="ui label  scope-parent" data-tooltip-content title=""><div class="ui label scope-left" style="color: #fff !important; background-color: #000000 !important">&lt;</div><div class="ui label scope-right" style="color: #fff !important; background-color: #000000 !important">&gt;</div></span>`
	assert.Equal(t, expectedLabel, string(ut.RenderLabel(label)))
	label = &issues.Label{ID: 123, Name: "</>", Exclusive: true, ExclusiveOrder: 1}
	expectedLabel = `<span class="ui label  scope-parent" data-tooltip-content title=""><div class="ui label scope-left" style="color: #fff !important; background-color: #000000 !important">&lt;</div><div class="ui label scope-middle" style="color: #fff !important; background-color: #000000 !important">&gt;</div><div class="ui label scope-right">1</div></span>`
	assert.Equal(t, expectedLabel, string(ut.RenderLabel(label)))
}

func TestUserMention(t *testing.T) {
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	rendered := newTestRenderUtils(t).MarkdownToHtml("@no-such-user @mention-user @mention-user")
	assert.Equal(t, `<p>@no-such-user <a href="/mention-user" rel="nofollow">@mention-user</a> <a href="/mention-user" rel="nofollow">@mention-user</a></p>`, strings.TrimSpace(string(rendered)))
}
