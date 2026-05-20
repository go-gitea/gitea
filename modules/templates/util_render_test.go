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
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
)

type stubDynGetter struct{}

func (stubDynGetter) GetValue(ctx context.Context, key string) (string, bool) {
	return "", false
}
func (stubDynGetter) GetRevision(ctx context.Context) int { return 0 }
func (stubDynGetter) InvalidateCache()                    {}

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
	if config.GetDynGetter() == nil {
		config.SetDynGetter(stubDynGetter{})
	}
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

	expectedLabel := `<a href="&lt;&gt;" class="ui label " style="color: #fff !important; background-color: label-color !important;" data-tooltip-content title=""><span class="gt-ellipsis">label-name</span></a>`
	assert.Equal(t, expectedLabel, string(ut.RenderLabelWithLink(label, "<>")))
	assert.Equal(t, expectedLabel, string(ut.RenderLabelWithLink(label, template.URL("<>"))))

	label = &issues.Label{ID: 123, Name: "</>", Exclusive: true}
	expectedLabel = `<a href="" class="ui label  scope-parent" data-tooltip-content title=""><div class="ui label scope-left" style="color: #fff !important; background-color: #000000 !important">&lt;</div><div class="ui label scope-right" style="color: #fff !important; background-color: #000000 !important">&gt;</div></a>`
	assert.Equal(t, expectedLabel, string(ut.RenderLabelWithLink(label, "")))
	label = &issues.Label{ID: 123, Name: "</>", Exclusive: true, ExclusiveOrder: 1}
	expectedLabel = `<a href="" class="ui label  scope-parent" data-tooltip-content title=""><div class="ui label scope-left" style="color: #fff !important; background-color: #000000 !important">&lt;</div><div class="ui label scope-middle" style="color: #fff !important; background-color: #000000 !important">&gt;</div><div class="ui label scope-right">1</div></a>`
	assert.Equal(t, expectedLabel, string(ut.RenderLabelWithLink(label, "")))
}

func TestUserMention(t *testing.T) {
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	rendered := newTestRenderUtils(t).MarkdownToHtml("@no-such-user @mention-user @mention-user")
	assert.Equal(t, `<p>@no-such-user <a href="/mention-user" rel="nofollow">@mention-user</a> <a href="/mention-user" rel="nofollow">@mention-user</a></p>`, strings.TrimSpace(string(rendered)))
}

func TestCoAuthorAvatars(t *testing.T) {
	ut := newTestRenderUtils(t)
	authorSig := &git.Signature{Name: "Alice", Email: "alice@example.com"}
	mkCo := func(name, email string) *user_model.CoAuthorUser {
		return &user_model.CoAuthorUser{TrailerSignature: &git.Signature{Name: name, Email: email}}
	}

	mkData := func(co []*user_model.CoAuthorUser) *user_model.CoAuthorAvatarData {
		return &user_model.CoAuthorAvatarData{AuthorSig: authorSig, CoAuthors: co}
	}

	t.Run("zero co-authors renders bare author, no label", func(t *testing.T) {
		got := string(ut.CoAuthorAvatars(mkData(nil)))
		assert.Contains(t, got, `<span class="author-wrapper">`)
		assert.Contains(t, got, "Alice")
		assert.NotContains(t, got, "coauthor_and")
		assert.NotContains(t, got, "coauthor_people")
	})

	t.Run("single co-author uses and label", func(t *testing.T) {
		got := string(ut.CoAuthorAvatars(mkData([]*user_model.CoAuthorUser{mkCo("Bob", "bob@example.com")})))
		assert.Contains(t, got, "repo.commits.coauthor_and")
		assert.Contains(t, got, "Bob")
		assert.NotContains(t, got, "coauthor_people")
		assert.Contains(t, got, `<span class="avatar-stack">`)
	})

	t.Run("two co-authors switches to N people label with tippy popup", func(t *testing.T) {
		got := string(ut.CoAuthorAvatars(mkData(
			[]*user_model.CoAuthorUser{mkCo("Bob", "bob@example.com"), mkCo("Carol", "carol@example.com")})))
		assert.Contains(t, got, "repo.commits.coauthor_people:3")
		assert.NotContains(t, got, "repo.commits.coauthor_and")
		assert.Contains(t, got, `data-global-init="initAuthorsPopup"`)
		assert.Contains(t, got, `<div class="tippy-target">`)
		assert.Contains(t, got, `class="authors-popup"`)
	})

	t.Run("overflow chip renders for >9 co-authors", func(t *testing.T) {
		cos := make([]*user_model.CoAuthorUser, 11)
		for i := range cos {
			cos[i] = mkCo("X", "x@example.com")
		}
		got := string(ut.CoAuthorAvatars(mkData(cos)))
		assert.Contains(t, got, `class="avatar-stack-overflow-chip`)
		assert.Contains(t, got, "+2")
		assert.Contains(t, got, "repo.commits.coauthor_people:12")
		assert.Contains(t, got, `data-global-init="initAuthorsPopup"`)
	})

	t.Run("chip alone renders for 10 co-authors", func(t *testing.T) {
		cos := make([]*user_model.CoAuthorUser, 10)
		for i := range cos {
			cos[i] = mkCo("X", "x@example.com")
		}
		got := string(ut.AvatarStack(nil, authorSig, cos))
		assert.Contains(t, got, `class="avatar-stack-overflow-chip`)
		assert.Contains(t, got, "+1")
	})
}
