// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"testing"
	texttmpl "text/template"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/attachment"
	sender_service "code.gitea.io/gitea/services/mailer/sender"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const subjectTpl = `
{{.SubjectPrefix}}[{{.Repo}}] @{{.Doer.Name}} #{{.Issue.Index}} - {{.Issue.Title}}
`

const bodyTpl = `
<!DOCTYPE html>
<html>
<head>
	<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
	<title>{{.Subject}}</title>
</head>

<body>
	<p>{{.Body}}</p>
	<p>
		---
		<br>
		<a href="{{.Link}}">View it on Gitea</a>.
	</p>
</body>
</html>
`

func prepareMailerTest(t *testing.T) (doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.MailService = &setting.Mailer{From: "test@gitea.com"}
	setting.Domain = "localhost"
	setting.AppURL = "https://try.gitea.io/"

	doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, Owner: doer})
	issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1, Repo: repo, Poster: doer})
	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2, Issue: issue})
	require.NoError(t, issue.LoadRepo(db.DefaultContext))
	return doer, repo, issue, comment
}

func prepareMailerBase64Test(t *testing.T) (doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, att1, att2 *repo_model.Attachment) {
	user, repo, issue, comment := prepareMailerTest(t)
	setting.MailService.EmbedAttachmentImages = true

	att1, err := attachment.NewAttachment(t.Context(), &repo_model.Attachment{
		RepoID:     repo.ID,
		IssueID:    issue.ID,
		UploaderID: user.ID,
		CommentID:  comment.ID,
		Name:       "test.png",
	}, bytes.NewReader([]byte("\x89\x50\x4e\x47\x0d\x0a\x1a\x0a")), 8)
	require.NoError(t, err)

	att2, err = attachment.NewAttachment(t.Context(), &repo_model.Attachment{
		RepoID:     repo.ID,
		IssueID:    issue.ID,
		UploaderID: user.ID,
		CommentID:  comment.ID,
		Name:       "test.png",
	}, bytes.NewReader([]byte("\x89\x50\x4e\x47\x0d\x0a\x1a\x0a"+strings.Repeat("\x00", 1024))), 8+1024)
	require.NoError(t, err)

	return user, repo, issue, att1, att2
}

func TestComposeIssueComment(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)

	markup.Init(&markup.RenderHelperFuncs{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == doer.Name
		},
	})

	setting.IncomingEmail.Enabled = true
	defer func() { setting.IncomingEmail.Enabled = false }()

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/comment").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/comment").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(t.Context(), &mailComment{
		Issue: issue, Doer: doer, ActionType: activities_model.ActionCommentIssue,
		Content: fmt.Sprintf("test @%s %s#%d body", doer.Name, issue.Repo.FullName(), issue.Index),
		Comment: comment,
	}, "en-US", recipients, false, "issue comment")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	gomailMsg := msgs[0].ToMessage()
	replyTo := gomailMsg.GetGenHeader("Reply-To")[0]
	subject := gomailMsg.GetGenHeader("Subject")[0]

	assert.Len(t, gomailMsg.GetAddrHeader("To"), 1, "exactly one recipient is expected in the To field")
	tokenRegex := regexp.MustCompile(`\Aincoming\+(.+)@localhost\z`)
	assert.Regexp(t, tokenRegex, replyTo)
	token := tokenRegex.FindAllStringSubmatch(replyTo, 1)[0][1]
	assert.Equal(t, "Re: ", subject[:4], "Comment reply subject should contain Re:")
	assert.Equal(t, "Re: [user2/repo1] @user2 #1 - issue1", subject)
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", gomailMsg.GetGenHeader("In-Reply-To")[0], "In-Reply-To header doesn't match")
	assert.ElementsMatch(t, []string{"<user2/repo1/issues/1@localhost>", "<reply-" + token + "@localhost>"}, gomailMsg.GetGenHeader("References"), "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1/comment/2@localhost>", gomailMsg.GetGenHeader("Message-ID")[0], "Message-ID header doesn't match")
	assert.Equal(t, "<mailto:"+replyTo+">", gomailMsg.GetGenHeader("List-Post")[0])
	assert.Len(t, gomailMsg.GetGenHeader("List-Unsubscribe"), 2) // url + mailto

	var buf bytes.Buffer
	_, err = gomailMsg.WriteTo(&buf)
	require.NoError(t, err)

	b, err := io.ReadAll(quotedprintable.NewReader(&buf))
	assert.NoError(t, err)

	// text/plain
	assert.Contains(t, string(b), fmt.Sprintf(`( %s )`, doer.HTMLURL()))
	assert.Contains(t, string(b), fmt.Sprintf(`( %s )`, issue.HTMLURL()))

	// text/html
	assert.Contains(t, string(b), fmt.Sprintf(`href="%s"`, doer.HTMLURL()))
	assert.Contains(t, string(b), fmt.Sprintf(`href="%s"`, issue.HTMLURL()))
}

func TestMailMentionsComment(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)
	comment.Poster = doer
	subjectTemplates = texttmpl.Must(texttmpl.New("issue/comment").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/comment").Parse(bodyTpl))
	mails := 0

	defer test.MockVariableValue(&SendAsync, func(msgs ...*sender_service.Message) {
		mails = len(msgs)
	})()

	err := MailParticipantsComment(t.Context(), comment, activities_model.ActionCommentIssue, issue, []*user_model.User{})
	require.NoError(t, err)
	assert.Equal(t, 3, mails)
}

func TestComposeIssueMessage(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/new").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(t.Context(), &mailComment{
		Issue: issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: "test body",
	}, "en-US", recipients, false, "issue create")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)

	gomailMsg := msgs[0].ToMessage()
	mailto := gomailMsg.GetAddrHeader("To")
	subject := gomailMsg.GetGenHeader("Subject")
	messageID := gomailMsg.GetGenHeader("Message-ID")
	inReplyTo := gomailMsg.GetGenHeader("In-Reply-To")
	references := gomailMsg.GetGenHeader("References")

	assert.Len(t, mailto, 1, "exactly one recipient is expected in the To field")
	assert.Equal(t, "[user2/repo1] @user2 #1 - issue1", subject[0])
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", inReplyTo[0], "In-Reply-To header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", references[0], "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", messageID[0], "Message-ID header doesn't match")
	assert.Empty(t, gomailMsg.GetGenHeader("List-Post"))         // incoming mail feature disabled
	assert.Len(t, gomailMsg.GetGenHeader("List-Unsubscribe"), 1) // url without mailto
}

func TestTemplateSelection(t *testing.T) {
	doer, repo, issue, comment := prepareMailerTest(t)
	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/default").Parse("issue/default/subject"))
	texttmpl.Must(subjectTemplates.New("issue/new").Parse("issue/new/subject"))
	texttmpl.Must(subjectTemplates.New("pull/comment").Parse("pull/comment/subject"))
	texttmpl.Must(subjectTemplates.New("issue/close").Parse("")) // Must default to fallback subject

	bodyTemplates = template.Must(template.New("issue/default").Parse("issue/default/body"))
	template.Must(bodyTemplates.New("issue/new").Parse("issue/new/body"))
	template.Must(bodyTemplates.New("pull/comment").Parse("pull/comment/body"))
	template.Must(bodyTemplates.New("issue/close").Parse("issue/close/body"))

	expect := func(t *testing.T, msg *sender_service.Message, expSubject, expBody string) {
		subject := msg.ToMessage().GetGenHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()
		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, expBody)
	}

	msg := testComposeIssueCommentMessage(t, &mailComment{
		Issue: issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: "test body",
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/new/subject", "issue/new/body")

	msg = testComposeIssueCommentMessage(t, &mailComment{
		Issue: issue, Doer: doer, ActionType: activities_model.ActionCommentIssue,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/default/subject", "issue/default/body")

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, Repo: repo, Poster: doer})
	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4, Issue: pull})
	msg = testComposeIssueCommentMessage(t, &mailComment{
		Issue: pull, Doer: doer, ActionType: activities_model.ActionCommentPull,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "pull/comment/subject", "pull/comment/body")

	msg = testComposeIssueCommentMessage(t, &mailComment{
		Issue: issue, Doer: doer, ActionType: activities_model.ActionCloseIssue,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "Re: [user2/repo1] issue1 (#1)", "issue/close/body")
}

func TestTemplateServices(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))

	expect := func(t *testing.T, issue *issues_model.Issue, comment *issues_model.Comment, doer *user_model.User,
		actionType activities_model.ActionType, fromMention bool, tplSubject, tplBody, expSubject, expBody string,
	) {
		subjectTemplates = texttmpl.Must(texttmpl.New("issue/default").Parse(tplSubject))
		bodyTemplates = template.Must(template.New("issue/default").Parse(tplBody))

		recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}
		msg := testComposeIssueCommentMessage(t, &mailComment{
			Issue: issue, Doer: doer, ActionType: actionType,
			Content: "test body", Comment: comment,
		}, recipients, fromMention, "TestTemplateServices")

		subject := msg.ToMessage().GetGenHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()

		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, "\r\n"+expBody+"\r\n")
	}

	expect(t, issue, comment, doer, activities_model.ActionCommentIssue, false,
		"{{.SubjectPrefix}}[{{.Repo}}]: @{{.Doer.Name}} commented on #{{.Issue.Index}} - {{.Issue.Title}}",
		"//{{.ActionType}},{{.ActionName}},{{if .IsMention}}norender{{end}}//",
		"Re: [user2/repo1]: @user2 commented on #1 - issue1",
		"//issue,comment,//")

	expect(t, issue, comment, doer, activities_model.ActionCommentIssue, true,
		"{{if .IsMention}}must render{{end}}",
		"//subject is: {{.Subject}}//",
		"must render",
		"//subject is: must render//")

	expect(t, issue, comment, doer, activities_model.ActionCommentIssue, true,
		"{{.FallbackSubject}}",
		"//{{.SubjectPrefix}}//",
		"Re: [user2/repo1] issue1 (#1)",
		"//Re: //")
}

func testComposeIssueCommentMessage(t *testing.T, ctx *mailComment, recipients []*user_model.User, fromMention bool, info string) *sender_service.Message {
	msgs, err := composeIssueCommentMessages(t.Context(), ctx, "en-US", recipients, fromMention, info)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	return msgs[0]
}

func TestGenerateAdditionalHeaders(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	comment := &mailComment{Issue: issue, Doer: doer}
	recipient := &user_model.User{Name: "test", Email: "test@gitea.com"}

	headers := generateAdditionalHeaders(comment, "dummy-reason", recipient)

	expected := map[string]string{
		"List-ID":                   "user2/repo1 <repo1.user2.localhost>",
		"List-Archive":              "<https://try.gitea.io/user2/repo1>",
		"X-Gitea-Reason":            "dummy-reason",
		"X-Gitea-Sender":            "user2",
		"X-Gitea-Recipient":         "test",
		"X-Gitea-Recipient-Address": "test@gitea.com",
		"X-Gitea-Repository":        "repo1",
		"X-Gitea-Repository-Path":   "user2/repo1",
		"X-Gitea-Repository-Link":   "https://try.gitea.io/user2/repo1",
		"X-Gitea-Issue-ID":          "1",
		"X-Gitea-Issue-Link":        "https://try.gitea.io/user2/repo1/issues/1",
	}

	for key, value := range expected {
		if assert.Contains(t, headers, key) {
			assert.Equal(t, value, headers[key])
		}
	}
}

func TestGenerateMessageIDForIssue(t *testing.T) {
	_, _, issue, comment := prepareMailerTest(t)
	_, _, pullIssue, _ := prepareMailerTest(t)
	pullIssue.IsPull = true

	type args struct {
		issue      *issues_model.Issue
		comment    *issues_model.Comment
		actionType activities_model.ActionType
	}
	tests := []struct {
		name   string
		args   args
		prefix string
	}{
		{
			name: "Open Issue",
			args: args{
				issue:      issue,
				actionType: activities_model.ActionCreateIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d@%s>", issue.Repo.FullName(), issue.Index, setting.Domain),
		},
		{
			name: "Open Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionCreatePullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d@%s>", issue.Repo.FullName(), issue.Index, setting.Domain),
		},
		{
			name: "Comment Issue",
			args: args{
				issue:      issue,
				comment:    comment,
				actionType: activities_model.ActionCommentIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d/comment/%d@%s>", issue.Repo.FullName(), issue.Index, comment.ID, setting.Domain),
		},
		{
			name: "Comment Pull",
			args: args{
				issue:      pullIssue,
				comment:    comment,
				actionType: activities_model.ActionCommentPull,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/comment/%d@%s>", issue.Repo.FullName(), issue.Index, comment.ID, setting.Domain),
		},
		{
			name: "Close Issue",
			args: args{
				issue:      issue,
				actionType: activities_model.ActionCloseIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d/close/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Close Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionClosePullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/close/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Reopen Issue",
			args: args{
				issue:      issue,
				actionType: activities_model.ActionReopenIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d/reopen/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Reopen Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionReopenPullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/reopen/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Merge Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionMergePullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/merge/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Ready Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionPullRequestReadyForReview,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/ready/", issue.Repo.FullName(), issue.Index),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateMessageIDForIssue(tt.args.issue, tt.args.comment, tt.args.actionType)
			assert.True(t, strings.HasPrefix(got, tt.prefix), "%v, want %v", got, tt.prefix)
		})
	}
}

func TestGenerateMessageIDForRelease(t *testing.T) {
	msgID := generateMessageIDForRelease(&repo_model.Release{
		ID:   1,
		Repo: &repo_model.Repository{OwnerName: "owner", Name: "repo"},
	})
	assert.Equal(t, "<owner/repo/releases/1@localhost>", msgID)
}

func TestFromDisplayName(t *testing.T) {
	tmpl, err := texttmpl.New("mailFrom").Parse("{{ .DisplayName }}")
	assert.NoError(t, err)
	setting.MailService = &setting.Mailer{FromDisplayNameFormatTemplate: tmpl}
	defer func() { setting.MailService = nil }()

	tests := []struct {
		userDisplayName string
		fromDisplayName string
	}{{
		userDisplayName: "test",
		fromDisplayName: "test",
	}, {
		userDisplayName: "Hi Its <Mee>",
		fromDisplayName: "Hi Its <Mee>",
	}, {
		userDisplayName: "Æsir",
		fromDisplayName: "=?utf-8?q?=C3=86sir?=",
	}, {
		userDisplayName: "new😀user",
		fromDisplayName: "=?utf-8?q?new=F0=9F=98=80user?=",
	}}

	for _, tc := range tests {
		t.Run(tc.userDisplayName, func(t *testing.T) {
			user := &user_model.User{FullName: tc.userDisplayName, Name: "tmp"}
			got := fromDisplayName(user)
			assert.EqualValues(t, tc.fromDisplayName, got)
		})
	}

	t.Run("template with all available vars", func(t *testing.T) {
		tmpl, err = texttmpl.New("mailFrom").Parse("{{ .DisplayName }} (by {{ .AppName }} on [{{ .Domain }}])")
		assert.NoError(t, err)
		setting.MailService = &setting.Mailer{FromDisplayNameFormatTemplate: tmpl}
		oldAppName := setting.AppName
		setting.AppName = "Code IT"
		oldDomain := setting.Domain
		setting.Domain = "code.it"
		defer func() {
			setting.AppName = oldAppName
			setting.Domain = oldDomain
		}()

		assert.EqualValues(t, "Mister X (by Code IT on [code.it])", fromDisplayName(&user_model.User{FullName: "Mister X", Name: "tmp"}))
	})
}

func TestEmbedBase64Images(t *testing.T) {
	user, repo, issue, att1, att2 := prepareMailerBase64Test(t)
	// comment := &mailComment{Issue: issue, Doer: user}

	imgExternalURL := "https://via.placeholder.com/10"
	imgExternalImg := fmt.Sprintf(`<img src="%s"/>`, imgExternalURL)

	att1URL := setting.AppURL + repo.Owner.Name + "/" + repo.Name + "/attachments/" + att1.UUID
	att1Img := fmt.Sprintf(`<img src="%s"/>`, att1URL)
	att1Base64 := "data:image/png;base64,iVBORw0KGgo="
	att1ImgBase64 := fmt.Sprintf(`<img src="%s"/>`, att1Base64)

	att2URL := setting.AppURL + repo.Owner.Name + "/" + repo.Name + "/attachments/" + att2.UUID
	att2Img := fmt.Sprintf(`<img src="%s"/>`, att2URL)
	att2File, err := storage.Attachments.Open(att2.RelativePath())
	require.NoError(t, err)
	defer att2File.Close()
	att2Bytes, err := io.ReadAll(att2File)
	require.NoError(t, err)
	require.Greater(t, len(att2Bytes), 1024)
	att2Base64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(att2Bytes)
	att2ImgBase64 := fmt.Sprintf(`<img src="%s"/>`, att2Base64)

	t.Run("ComposeMessage", func(t *testing.T) {
		subjectTemplates = texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
		bodyTemplates = template.Must(template.New("issue/new").Parse(bodyTpl))

		issue.Content = fmt.Sprintf(`MSG-BEFORE <image src="attachments/%s"> MSG-AFTER`, att1.UUID)
		require.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "content"))

		recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}
		msgs, err := composeIssueCommentMessages(t.Context(), &mailComment{
			Issue:      issue,
			Doer:       user,
			ActionType: activities_model.ActionCreateIssue,
			Content:    issue.Content,
		}, "en-US", recipients, false, "issue create")
		require.NoError(t, err)

		mailBody := msgs[0].Body
		assert.Regexp(t, `MSG-BEFORE <a[^>]+><img src="data:image/png;base64,iVBORw0KGgo="/></a> MSG-AFTER`, mailBody)
	})

	t.Run("EmbedInstanceImageSkipExternalImage", func(t *testing.T) {
		mailBody := "<html><head></head><body><p>Test1</p>" + imgExternalImg + "<p>Test2</p>" + att1Img + "<p>Test3</p></body></html>"
		expectedMailBody := "<html><head></head><body><p>Test1</p>" + imgExternalImg + "<p>Test2</p>" + att1ImgBase64 + "<p>Test3</p></body></html>"
		b64embedder := newMailAttachmentBase64Embedder(user, repo, 1024)
		resultMailBody, err := b64embedder.Base64InlineImages(t.Context(), template.HTML(mailBody))
		require.NoError(t, err)
		assert.Equal(t, expectedMailBody, string(resultMailBody))
	})

	t.Run("LimitedEmailBodySize", func(t *testing.T) {
		mailBody := fmt.Sprintf("<html><head></head><body>%s%s</body></html>", att1Img, att2Img)
		b64embedder := newMailAttachmentBase64Embedder(user, repo, 1024)
		resultMailBody, err := b64embedder.Base64InlineImages(t.Context(), template.HTML(mailBody))
		require.NoError(t, err)
		expected := fmt.Sprintf("<html><head></head><body>%s%s</body></html>", att1ImgBase64, att2Img)
		assert.Equal(t, expected, string(resultMailBody))

		b64embedder = newMailAttachmentBase64Embedder(user, repo, 4096)
		resultMailBody, err = b64embedder.Base64InlineImages(t.Context(), template.HTML(mailBody))
		require.NoError(t, err)
		expected = fmt.Sprintf("<html><head></head><body>%s%s</body></html>", att1ImgBase64, att2ImgBase64)
		assert.Equal(t, expected, string(resultMailBody))
	})
}
