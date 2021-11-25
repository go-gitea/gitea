// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"html/template"
	"testing"
	texttmpl "text/template"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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

func prepareMailerTest(t *testing.T) (doer *user_model.User, repo *models.Repository, issue *models.Issue, comment *models.Comment) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	var mailService = setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo = unittest.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue = unittest.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)
	assert.NoError(t, issue.LoadRepo())
	comment = unittest.AssertExistsAndLoadBean(t, &models.Comment{ID: 2, Issue: issue}).(*models.Comment)
	return
}

func TestComposeIssueCommentMessage(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)

	stpl := texttmpl.Must(texttmpl.New("issue/comment").Parse(subjectTpl))
	btpl := template.Must(template.New("issue/comment").Parse(bodyTpl))
	InitMailRender(stpl, btpl)

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{Issue: issue, Doer: doer, ActionType: models.ActionCommentIssue,
		Content: "test body", Comment: comment}, "en-US", recipients, false, "issue comment")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	gomailMsg := msgs[0].ToMessage()
	mailto := gomailMsg.GetHeader("To")
	subject := gomailMsg.GetHeader("Subject")
	messageID := gomailMsg.GetHeader("Message-ID")
	inReplyTo := gomailMsg.GetHeader("In-Reply-To")
	references := gomailMsg.GetHeader("References")

	assert.Len(t, mailto, 1, "exactly one recipient is expected in the To field")
	assert.Equal(t, "Re: ", subject[0][:4], "Comment reply subject should contain Re:")
	assert.Equal(t, "Re: [user2/repo1] @user2 #1 - issue1", subject[0])
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", inReplyTo[0], "In-Reply-To header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", references[0], "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1/comment/2@localhost>", messageID[0], "Message-ID header doesn't match")
}

func TestComposeIssueMessage(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	stpl := texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
	btpl := template.Must(template.New("issue/new").Parse(bodyTpl))
	InitMailRender(stpl, btpl)

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{Issue: issue, Doer: doer, ActionType: models.ActionCreateIssue,
		Content: "test body"}, "en-US", recipients, false, "issue create")
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
}

func TestTemplateSelection(t *testing.T) {
	doer, repo, issue, comment := prepareMailerTest(t)
	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}

	stpl := texttmpl.Must(texttmpl.New("issue/default").Parse("issue/default/subject"))
	texttmpl.Must(stpl.New("issue/new").Parse("issue/new/subject"))
	texttmpl.Must(stpl.New("pull/comment").Parse("pull/comment/subject"))
	texttmpl.Must(stpl.New("issue/close").Parse("")) // Must default to fallback subject

	btpl := template.Must(template.New("issue/default").Parse("issue/default/body"))
	template.Must(btpl.New("issue/new").Parse("issue/new/body"))
	template.Must(btpl.New("pull/comment").Parse("pull/comment/body"))
	template.Must(btpl.New("issue/close").Parse("issue/close/body"))

	InitMailRender(stpl, btpl)

	expect := func(t *testing.T, msg *Message, expSubject, expBody string) {
		subject := msg.ToMessage().GetHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()
		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, expBody)
	}

	msg := testComposeIssueCommentMessage(t, &mailCommentContext{Issue: issue, Doer: doer, ActionType: models.ActionCreateIssue,
		Content: "test body"}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/new/subject", "issue/new/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{Issue: issue, Doer: doer, ActionType: models.ActionCommentIssue,
		Content: "test body", Comment: comment}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/default/subject", "issue/default/body")

	pull := unittest.AssertExistsAndLoadBean(t, &models.Issue{ID: 2, Repo: repo, Poster: doer}).(*models.Issue)
	comment = unittest.AssertExistsAndLoadBean(t, &models.Comment{ID: 4, Issue: pull}).(*models.Comment)
	msg = testComposeIssueCommentMessage(t, &mailCommentContext{Issue: pull, Doer: doer, ActionType: models.ActionCommentPull,
		Content: "test body", Comment: comment}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "pull/comment/subject", "pull/comment/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{Issue: issue, Doer: doer, ActionType: models.ActionCloseIssue,
		Content: "test body", Comment: comment}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "Re: [user2/repo1] issue1 (#1)", "issue/close/body")
}

func TestTemplateServices(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)
	assert.NoError(t, issue.LoadRepo())

	expect := func(t *testing.T, issue *models.Issue, comment *models.Comment, doer *user_model.User,
		actionType models.ActionType, fromMention bool, tplSubject, tplBody, expSubject, expBody string) {

		stpl := texttmpl.Must(texttmpl.New("issue/default").Parse(tplSubject))
		btpl := template.Must(template.New("issue/default").Parse(tplBody))
		InitMailRender(stpl, btpl)

		recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}
		msg := testComposeIssueCommentMessage(t, &mailCommentContext{Issue: issue, Doer: doer, ActionType: actionType,
			Content: "test body", Comment: comment}, recipients, fromMention, "TestTemplateServices")

		subject := msg.ToMessage().GetHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()

		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, "\r\n"+expBody+"\r\n")
	}

	expect(t, issue, comment, doer, models.ActionCommentIssue, false,
		"{{.SubjectPrefix}}[{{.Repo}}]: @{{.Doer.Name}} commented on #{{.Issue.Index}} - {{.Issue.Title}}",
		"//{{.ActionType}},{{.ActionName}},{{if .IsMention}}norender{{end}}//",
		"Re: [user2/repo1]: @user2 commented on #1 - issue1",
		"//issue,comment,//")

	expect(t, issue, comment, doer, models.ActionCommentIssue, true,
		"{{if .IsMention}}must render{{end}}",
		"//subject is: {{.Subject}}//",
		"must render",
		"//subject is: must render//")

	expect(t, issue, comment, doer, models.ActionCommentIssue, true,
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

	ctx := &mailCommentContext{Issue: issue, Doer: doer}
	recipient := &user_model.User{Name: "Test", Email: "test@gitea.com"}

	headers := generateAdditionalHeaders(ctx, "dummy-reason", recipient)

	expected := map[string]string{
		"List-ID":                   "user2/repo1 <repo1.user2.localhost>",
		"List-Archive":              "<https://try.gitea.io/user2/repo1>",
		"X-Gitea-Reason":            "dummy-reason",
		"X-Gitea-Sender":            "< U<se>r Tw<o > ><",
		"X-Gitea-Recipient":         "Test",
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
