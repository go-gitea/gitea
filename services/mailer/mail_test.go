// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
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

	"github.com/stretchr/testify/assert"
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
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, Owner: doer})
	issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1, Repo: repo, Poster: doer})
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))
	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2, Issue: issue})
	return doer, repo, issue, comment
}

func TestComposeIssueCommentMessage(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)

	markup.Init(&markup.ProcessorHelper{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == doer.Name
		},
	})

	setting.IncomingEmail.Enabled = true
	defer func() { setting.IncomingEmail.Enabled = false }()

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/comment").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/comment").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCommentIssue,
		Content: fmt.Sprintf("test @%s %s#%d body", doer.Name, issue.Repo.FullName(), issue.Index),
		Comment: comment,
	}, "en-US", recipients, false, "issue comment")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	gomailMsg := msgs[0].ToMessage()
	replyTo := gomailMsg.GetHeader("Reply-To")[0]
	subject := gomailMsg.GetHeader("Subject")[0]

	assert.Len(t, gomailMsg.GetHeader("To"), 1, "exactly one recipient is expected in the To field")
	tokenRegex := regexp.MustCompile(`\Aincoming\+(.+)@localhost\z`)
	assert.Regexp(t, tokenRegex, replyTo)
	token := tokenRegex.FindAllStringSubmatch(replyTo, 1)[0][1]
	assert.Equal(t, "Re: ", subject[:4], "Comment reply subject should contain Re:")
	assert.Equal(t, "Re: [user2/repo1] @user2 #1 - issue1", subject)
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", gomailMsg.GetHeader("In-Reply-To")[0], "In-Reply-To header doesn't match")
	assert.ElementsMatch(t, []string{"<user2/repo1/issues/1@localhost>", "<reply-" + token + "@localhost>"}, gomailMsg.GetHeader("References"), "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1/comment/2@localhost>", gomailMsg.GetHeader("Message-ID")[0], "Message-ID header doesn't match")
	assert.Equal(t, "<mailto:"+replyTo+">", gomailMsg.GetHeader("List-Post")[0])
	assert.Len(t, gomailMsg.GetHeader("List-Unsubscribe"), 2) // url + mailto

	var buf bytes.Buffer
	gomailMsg.WriteTo(&buf)

	b, err := io.ReadAll(quotedprintable.NewReader(&buf))
	assert.NoError(t, err)

	// text/plain
	assert.Contains(t, string(b), fmt.Sprintf(`( %s )`, doer.HTMLURL()))
	assert.Contains(t, string(b), fmt.Sprintf(`( %s )`, issue.HTMLURL()))

	// text/html
	assert.Contains(t, string(b), fmt.Sprintf(`href="%s"`, doer.HTMLURL()))
	assert.Contains(t, string(b), fmt.Sprintf(`href="%s"`, issue.HTMLURL()))
}

func TestComposeIssueMessage(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/new").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: "test body",
	}, "en-US", recipients, false, "issue create")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)

	gomailMsg := msgs[0].ToMessage()
	mailto := gomailMsg.GetHeader("To")
	subject := gomailMsg.GetHeader("Subject")
	messageID := gomailMsg.GetHeader("Message-ID")
	inReplyTo := gomailMsg.GetHeader("In-Reply-To")
	references := gomailMsg.GetHeader("References")

	assert.Len(t, mailto, 1, "exactly one recipient is expected in the To field")
	assert.Equal(t, "[user2/repo1] @user2 #1 - issue1", subject[0])
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", inReplyTo[0], "In-Reply-To header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", references[0], "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", messageID[0], "Message-ID header doesn't match")
	assert.Empty(t, gomailMsg.GetHeader("List-Post"))         // incoming mail feature disabled
	assert.Len(t, gomailMsg.GetHeader("List-Unsubscribe"), 1) // url without mailto
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

	expect := func(t *testing.T, msg *Message, expSubject, expBody string) {
		subject := msg.ToMessage().GetHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()
		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, expBody)
	}

	msg := testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: "test body",
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/new/subject", "issue/new/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCommentIssue,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/default/subject", "issue/default/body")

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, Repo: repo, Poster: doer})
	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4, Issue: pull})
	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   pull, Doer: doer, ActionType: activities_model.ActionCommentPull,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "pull/comment/subject", "pull/comment/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCloseIssue,
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
		msg := testComposeIssueCommentMessage(t, &mailCommentContext{
			Context: context.TODO(), // TODO: use a correct context
			Issue:   issue, Doer: doer, ActionType: actionType,
			Content: "test body", Comment: comment,
		}, recipients, fromMention, "TestTemplateServices")

		subject := msg.ToMessage().GetHeader("Subject")
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

func testComposeIssueCommentMessage(t *testing.T, ctx *mailCommentContext, recipients []*user_model.User, fromMention bool, info string) *Message {
	msgs, err := composeIssueCommentMessages(ctx, "en-US", recipients, fromMention, info)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	return msgs[0]
}

func TestGenerateAdditionalHeaders(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	ctx := &mailCommentContext{Context: context.TODO() /* TODO: use a correct context */, Issue: issue, Doer: doer}
	recipient := &user_model.User{Name: "test", Email: "test@gitea.com"}

	headers := generateAdditionalHeaders(ctx, "dummy-reason", recipient)

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
			if !strings.HasPrefix(got, tt.prefix) {
				t.Errorf("generateMessageIDForIssue() = %v, want %v", got, tt.prefix)
			}
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
