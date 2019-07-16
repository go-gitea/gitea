// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"html/template"
	"testing"

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
	assert.NoError(t, PrepareTestDatabase())
	var MailService setting.Mailer

	MailService.From = "test@gitea.com"
	setting.MailService = &MailService

	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1, Owner: doer}).(*Repository)
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1, Repo: repo, Poster: doer}).(*Issue)
	comment := AssertExistsAndLoadBean(t, &Comment{ID: 2, Issue: issue}).(*Comment)

	email := template.Must(template.New("issue/comment").Parse(tmpl))
	InitMailRender(email)

	tos := []string{"test@gitea.com", "test2@gitea.com"}
	msg := composeIssueCommentMessage(issue, doer, "test body", comment, mailIssueComment, tos, "issue comment")
	subject := msg.GetHeader("Subject")
	assert.Equal(t, subject[0], "Re: "+issue.mailSubject(), "Comment reply subject should contain Re:")

}

func TestComposeIssueMessage(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	var MailService setting.Mailer

	MailService.From = "test@gitea.com"
	setting.MailService = &MailService

	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1, Owner: doer}).(*Repository)
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1, Repo: repo, Poster: doer}).(*Issue)

	email := template.Must(template.New("issue/comment").Parse(tmpl))
	InitMailRender(email)

	tos := []string{"test@gitea.com", "test2@gitea.com"}
	msg := composeIssueCommentMessage(issue, doer, "test body", nil, mailIssueComment, tos, "issue create")
	subject := msg.GetHeader("Subject")
	assert.Equal(t, subject[0], issue.mailSubject(), "Subject not equal to issue.mailSubject()")
}
