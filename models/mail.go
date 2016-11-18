// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"html/template"
	"path"

	"gopkg.in/gomail.v2"
	"gopkg.in/macaron.v1"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/mailer"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
)

const (
	MailAuthActivate       base.TplName = "auth/activate"
	MailAuthActivateEmail  base.TplName = "auth/activate_email"
	MailAuthResetPassword  base.TplName = "auth/reset_passwd"
	MailAuthRegisterNotify base.TplName = "auth/register_notify"

	MailIssueComment base.TplName = "issue/comment"
	MailIssueMention base.TplName = "issue/mention"

	MailNotifyCollaborator base.TplName = "notify/collaborator"
)

type MailRender interface {
	HTMLString(string, interface{}, ...macaron.HTMLOptions) (string, error)
}

var mailRender MailRender

func InitMailRender(dir, appendDir string, funcMap []template.FuncMap) {
	opt := &macaron.RenderOptions{
		Directory:         dir,
		AppendDirectories: []string{appendDir},
		Funcs:             funcMap,
		Extensions:        []string{".tmpl", ".html"},
	}
	ts := macaron.NewTemplateSet()
	ts.Set(macaron.DEFAULT_TPL_SET_NAME, opt)

	mailRender = &macaron.TplRender{
		TemplateSet: ts,
		Opt:         opt,
	}
}

func SendTestMail(email string) error {
	return gomail.Send(&mailer.Sender{}, mailer.NewMessage([]string{email}, "Gogs Test Email!", "Gogs Test Email!").Message)
}

func SendUserMail(c *macaron.Context, u *User, tpl base.TplName, code, subject, info string) {
	data := map[string]interface{}{
		"Username":          u.DisplayName(),
		"ActiveCodeLives":   setting.Service.ActiveCodeLives / 60,
		"ResetPwdCodeLives": setting.Service.ResetPwdCodeLives / 60,
		"Code":              code,
	}
	body, err := mailRender.HTMLString(string(tpl), data)
	if err != nil {
		log.Error(3, "HTMLString: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{u.Email}, subject, body)
	msg.Info = fmt.Sprintf("UID: %d, %s", u.ID, info)

	mailer.SendAsync(msg)
}

func SendActivateAccountMail(c *macaron.Context, u *User) {
	SendUserMail(c, u, MailAuthActivate, u.GenerateActivateCode(), c.Tr("mail.activate_account"), "activate account")
}

func SendResetPasswordMail(c *macaron.Context, u *User) {
	SendUserMail(c, u, MailAuthResetPassword, u.GenerateActivateCode(), c.Tr("mail.reset_password"), "reset password")
}

// SendActivateAccountMail sends confirmation email.
func SendActivateEmailMail(c *macaron.Context, u *User, email *EmailAddress) {
	data := map[string]interface{}{
		"Username":        u.DisplayName(),
		"ActiveCodeLives": setting.Service.ActiveCodeLives / 60,
		"Code":            u.GenerateEmailActivateCode(email.Email),
		"Email":           email.Email,
	}
	body, err := mailRender.HTMLString(string(MailAuthActivateEmail), data)
	if err != nil {
		log.Error(3, "HTMLString: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{email.Email}, c.Tr("mail.activate_email"), body)
	msg.Info = fmt.Sprintf("UID: %d, activate email", u.ID)

	mailer.SendAsync(msg)
}

// SendRegisterNotifyMail triggers a notify e-mail by admin created a account.
func SendRegisterNotifyMail(c *macaron.Context, u *User) {
	data := map[string]interface{}{
		"Username": u.DisplayName(),
	}
	body, err := mailRender.HTMLString(string(MailAuthRegisterNotify), data)
	if err != nil {
		log.Error(3, "HTMLString: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{u.Email}, c.Tr("mail.register_notify"), body)
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
	body, err := mailRender.HTMLString(string(MailNotifyCollaborator), data)
	if err != nil {
		log.Error(3, "HTMLString: %v", err)
		return
	}

	msg := mailer.NewMessage([]string{u.Email}, subject, body)
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

func composeIssueMessage(issue *Issue, doer *User, tplName base.TplName, tos []string, info string) *mailer.Message {
	subject := issue.MailSubject()
	body := string(markdown.RenderSpecialLink([]byte(issue.Content), issue.Repo.HTMLURL(), issue.Repo.ComposeMetas()))
	data := composeTplData(subject, body, issue.HTMLURL())
	data["Doer"] = doer
	content, err := mailRender.HTMLString(string(tplName), data)
	if err != nil {
		log.Error(3, "HTMLString (%s): %v", tplName, err)
	}
	msg := mailer.NewMessageFrom(tos, fmt.Sprintf(`"%s" <%s>`, doer.DisplayName(), setting.MailService.FromEmail), subject, content)
	msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)
	return msg
}

// SendIssueCommentMail composes and sends issue comment emails to target receivers.
func SendIssueCommentMail(issue *Issue, doer *User, tos []string) {
	if len(tos) == 0 {
		return
	}

	mailer.SendAsync(composeIssueMessage(issue, doer, MailIssueComment, tos, "issue comment"))
}

// SendIssueMentionMail composes and sends issue mention emails to target receivers.
func SendIssueMentionMail(issue *Issue, doer *User, tos []string) {
	if len(tos) == 0 {
		return
	}
	mailer.SendAsync(composeIssueMessage(issue, doer, MailIssueMention, tos, "issue mention"))
}
