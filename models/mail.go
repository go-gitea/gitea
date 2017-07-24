// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"fmt"
	"html/template"
	"path"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/mailer"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
	"gopkg.in/gomail.v2"
	"gopkg.in/macaron.v1"
)

const (
	mailAuthActivate       base.TplName = "auth/activate"
	mailAuthActivateEmail  base.TplName = "auth/activate_email"
	mailAuthResetPassword  base.TplName = "auth/reset_passwd"
	mailAuthRegisterNotify base.TplName = "auth/register_notify"

	mailIssueComment base.TplName = "issue/comment"
	mailIssueMention base.TplName = "issue/mention"

	mailNotifyCollaborator base.TplName = "notify/collaborator"
)

var templates *template.Template

// InitMailRender initializes the macaron mail renderer
func InitMailRender(tmpls *template.Template) {
	templates = tmpls
}

// SendTestMail sends a test mail
func SendTestMail(email string) error {
	return gomail.Send(mailer.Sender, mailer.NewMessage([]string{email}, "Gitea Test Email!", "Gitea Test Email!").Message)
}

// SendUserMail sends a mail to the user
func SendUserMail(c *macaron.Context, u *User, tpl base.TplName, code, subject, info string) {
	data := map[string]interface{}{
		"Username":          u.DisplayName(),
		"ActiveCodeLives":   base.MinutesToFriendly(setting.Service.ActiveCodeLives, c.Locale.Language()),
		"ResetPwdCodeLives": base.MinutesToFriendly(setting.Service.ResetPwdCodeLives, c.Locale.Language()),
		"Code":              code,
	}

	var content bytes.Buffer

	if err := templates.ExecuteTemplate(&content, string(tpl), data); err != nil {
		log.Error(3, "Template: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{u.Email}, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, %s", u.ID, info)

	mailer.SendAsync(msg)
}

// SendActivateAccountMail sends an activation mail to the user (new user registration)
func SendActivateAccountMail(c *macaron.Context, u *User) {
	SendUserMail(c, u, mailAuthActivate, u.GenerateActivateCode(), c.Tr("mail.activate_account"), "activate account")
}

// SendResetPasswordMail sends a password reset mail to the user
func SendResetPasswordMail(c *macaron.Context, u *User) {
	SendUserMail(c, u, mailAuthResetPassword, u.GenerateActivateCode(), c.Tr("mail.reset_password"), "reset password")
}

// SendActivateEmailMail sends confirmation email to confirm new email address
func SendActivateEmailMail(c *macaron.Context, u *User, email *EmailAddress) {
	data := map[string]interface{}{
		"Username":        u.DisplayName(),
		"ActiveCodeLives": base.MinutesToFriendly(setting.Service.ActiveCodeLives, c.Locale.Language()),
		"Code":            u.GenerateEmailActivateCode(email.Email),
		"Email":           email.Email,
	}

	var content bytes.Buffer

	if err := templates.ExecuteTemplate(&content, string(mailAuthActivateEmail), data); err != nil {
		log.Error(3, "Template: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{email.Email}, c.Tr("mail.activate_email"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, activate email", u.ID)

	mailer.SendAsync(msg)
}

// SendRegisterNotifyMail triggers a notify e-mail by admin created a account.
func SendRegisterNotifyMail(c *macaron.Context, u *User) {
	data := map[string]interface{}{
		"Username": u.DisplayName(),
	}

	var content bytes.Buffer

	if err := templates.ExecuteTemplate(&content, string(mailAuthRegisterNotify), data); err != nil {
		log.Error(3, "Template: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{u.Email}, c.Tr("mail.register_notify"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, registration notify", u.ID)

	mailer.SendAsync(msg)
}

// SendCollaboratorMail sends mail notification to new collaborator.
func SendCollaboratorMail(u, doer *User, repo *Repository) {
	repoName := path.Join(repo.Owner.Name, repo.Name)
	subject := fmt.Sprintf("%s added you to %s", doer.DisplayName(), repoName)

	data := map[string]interface{}{
		"Subject":  subject,
		"RepoName": repoName,
		"Link":     repo.HTMLURL(),
	}

	var content bytes.Buffer

	if err := templates.ExecuteTemplate(&content, string(mailNotifyCollaborator), data); err != nil {
		log.Error(3, "Template: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{u.Email}, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, add collaborator", u.ID)

	mailer.SendAsync(msg)
}

func composeTplData(subject, body, link string) map[string]interface{} {
	data := make(map[string]interface{}, 10)
	data["Subject"] = subject
	data["Body"] = body
	data["Link"] = link
	return data
}

func composeIssueCommentMessage(issue *Issue, doer *User, comment *Comment, tplName base.TplName, tos []string, info string) *mailer.Message {
	subject := issue.mailSubject()
	body := string(markdown.RenderString(issue.Content, issue.Repo.HTMLURL(), issue.Repo.ComposeMetas()))

	data := make(map[string]interface{}, 10)
	if comment != nil {
		data = composeTplData(subject, body, issue.HTMLURL()+"#"+comment.HashTag())
	} else {
		data = composeTplData(subject, body, issue.HTMLURL())
	}
	data["Doer"] = doer

	var content bytes.Buffer

	if err := templates.ExecuteTemplate(&content, string(tplName), data); err != nil {
		log.Error(3, "Template: %v", err)
	}

	msg := mailer.NewMessageFrom(tos, fmt.Sprintf(`"%s" <%s>`, doer.DisplayName(), setting.MailService.FromEmail), subject, content.String())
	msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)
	return msg
}

// SendIssueCommentMail composes and sends issue comment emails to target receivers.
func SendIssueCommentMail(issue *Issue, doer *User, comment *Comment, tos []string) {
	if len(tos) == 0 {
		return
	}

	mailer.SendAsync(composeIssueCommentMessage(issue, doer, comment, mailIssueComment, tos, "issue comment"))
}

// SendIssueMentionMail composes and sends issue mention emails to target receivers.
func SendIssueMentionMail(issue *Issue, doer *User, comment *Comment, tos []string) {
	if len(tos) == 0 {
		return
	}
	mailer.SendAsync(composeIssueCommentMessage(issue, doer, comment, mailIssueMention, tos, "issue mention"))
}
