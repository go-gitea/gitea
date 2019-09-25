// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"html/template"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const tmpl = `
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

func TestComposeIssueCommentMessage(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	var mailService = setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)
	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 2, Issue: issue}).(*models.Comment)

	email := template.Must(template.New("issue/comment").Parse(tmpl))
	InitMailRender(email)

	tos := []string{"test@gitea.com", "test2@gitea.com"}
	msg := composeIssueCommentMessage(issue, doer, "test body", comment, mailIssueComment, tos, "issue comment")

	subject := msg.GetHeader("Subject")
	inreplyTo := msg.GetHeader("In-Reply-To")
	references := msg.GetHeader("References")

	assert.Equal(t, subject[0], "Re: "+mailSubject(issue), "Comment reply subject should contain Re:")
	assert.Equal(t, inreplyTo[0], "<user2/repo1/issues/1@localhost>", "In-Reply-To header doesn't match")
	assert.Equal(t, references[0], "<user2/repo1/issues/1@localhost>", "References header doesn't match")
}

func TestComposeIssueMessage(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	var mailService = setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)

	email := template.Must(template.New("issue/comment").Parse(tmpl))
	InitMailRender(email)

	tos := []string{"test@gitea.com", "test2@gitea.com"}
	msg := composeIssueCommentMessage(issue, doer, "test body", nil, mailIssueComment, tos, "issue create")

	subject := msg.GetHeader("Subject")
	messageID := msg.GetHeader("Message-ID")

	assert.Equal(t, subject[0], mailSubject(issue), "Subject not equal to issue.mailSubject()")
	assert.Nil(t, msg.GetHeader("In-Reply-To"))
	assert.Nil(t, msg.GetHeader("References"))
	assert.Equal(t, messageID[0], "<user2/repo1/issues/1@localhost>", "Message-ID header doesn't match")
}
