// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"mime"
	"path"
	"regexp"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"gopkg.in/gomail.v2"
)

const (
	mailAuthActivate       base.TplName = "auth/activate"
	mailAuthActivateEmail  base.TplName = "auth/activate_email"
	mailAuthResetPassword  base.TplName = "auth/reset_passwd"
	mailAuthRegisterNotify base.TplName = "auth/register_notify"

	mailIssueDefault base.TplName = "issue/default"
	mailNewIssue     base.TplName = "issue/new"
	mailCommentIssue base.TplName = "issue/comment"
	mailCloseIssue   base.TplName = "issue/close"
	mailReopenIssue  base.TplName = "issue/reopen"

	mailPullRequestDefault base.TplName = "pull/default"
	mailNewPullRequest     base.TplName = "pull/new"
	mailCommentPullRequest base.TplName = "pull/comment"
	mailClosePullRequest   base.TplName = "pull/close"
	mailReopenPullRequest  base.TplName = "pull/reopen"
	mailMergePullRequest   base.TplName = "pull/merge" // FIXME: Where can I use this?

	mailNotifyCollaborator base.TplName = "notify/collaborator"

	// There's no actual limit for subject in RFC 5322
	mailMaxSubjectRunes = 256
)

var (
	bodyTemplates       *template.Template
	subjectTemplates    *texttmpl.Template
	subjectRemoveSpaces = regexp.MustCompile(`[\s]+`)
)

// InitMailRender initializes the mail renderer
func InitMailRender(subjectTpl *texttmpl.Template, bodyTpl *template.Template) {
	subjectTemplates = subjectTpl
	bodyTemplates = bodyTpl
}

// SendTestMail sends a test mail
func SendTestMail(email string) error {
	return gomail.Send(Sender, NewMessage([]string{email}, "Gitea Test Email!", "Gitea Test Email!").Message)
}

// SendUserMail sends a mail to the user
func SendUserMail(language string, u *models.User, tpl base.TplName, code, subject, info string) {
	data := map[string]interface{}{
		"DisplayName":       u.DisplayName(),
		"ActiveCodeLives":   timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, language),
		"ResetPwdCodeLives": timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, language),
		"Code":              code,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(tpl), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{u.Email}, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, %s", u.ID, info)

	SendAsync(msg)
}

// Locale represents an interface to translation
type Locale interface {
	Language() string
	Tr(string, ...interface{}) string
}

// SendActivateAccountMail sends an activation mail to the user (new user registration)
func SendActivateAccountMail(locale Locale, u *models.User) {
	SendUserMail(locale.Language(), u, mailAuthActivate, u.GenerateActivateCode(), locale.Tr("mail.activate_account"), "activate account")
}

// SendResetPasswordMail sends a password reset mail to the user
func SendResetPasswordMail(locale Locale, u *models.User) {
	SendUserMail(locale.Language(), u, mailAuthResetPassword, u.GenerateActivateCode(), locale.Tr("mail.reset_password"), "recover account")
}

// SendActivateEmailMail sends confirmation email to confirm new email address
func SendActivateEmailMail(locale Locale, u *models.User, email *models.EmailAddress) {
	data := map[string]interface{}{
		"DisplayName":     u.DisplayName(),
		"ActiveCodeLives": timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale.Language()),
		"Code":            u.GenerateEmailActivateCode(email.Email),
		"Email":           email.Email,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthActivateEmail), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{email.Email}, locale.Tr("mail.activate_email"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, activate email", u.ID)

	SendAsync(msg)
}

// SendRegisterNotifyMail triggers a notify e-mail by admin created a account.
func SendRegisterNotifyMail(locale Locale, u *models.User) {
	if setting.MailService == nil {
		log.Warn("SendRegisterNotifyMail is being invoked but mail service hasn't been initialized")
		return
	}

	data := map[string]interface{}{
		"DisplayName": u.DisplayName(),
		"Username":    u.Name,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthRegisterNotify), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{u.Email}, locale.Tr("mail.register_notify"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, registration notify", u.ID)

	SendAsync(msg)
}

// SendCollaboratorMail sends mail notification to new collaborator.
func SendCollaboratorMail(u, doer *models.User, repo *models.Repository) {
	repoName := path.Join(repo.Owner.Name, repo.Name)
	subject := fmt.Sprintf("%s added you to %s", doer.DisplayName(), repoName)

	data := map[string]interface{}{
		"Subject":  subject,
		"RepoName": repoName,
		"Link":     repo.HTMLURL(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailNotifyCollaborator), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{u.Email}, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, add collaborator", u.ID)

	SendAsync(msg)
}

func composeIssueCommentMessage(issue *models.Issue, doer *models.User, actionType models.ActionType, fromMention bool,
	content string, comment *models.Comment, tos []string, info string) *Message {

	if err := issue.LoadRepo(); err != nil {
		log.Error("LoadRepo: %v", err)
		return nil
	}
	if err := issue.LoadPullRequest(); err != nil {
		log.Error("LoadPullRequest: %v", err)
		return nil
	}

	var (
		subject string
		link    string
		prefix  string
		// Fall back subject for bad templates, make sure subject is never empty
		fallback string
	)

	if comment != nil {
		prefix = "Re: "
	}

	fallback = prefix + fallbackMailSubject(issue)

	// This is the body of the new issue or comment, not the mail body
	body := string(markup.RenderByType(markdown.MarkupName, []byte(content), issue.Repo.HTMLURL(), issue.Repo.ComposeMetas()))

	if comment != nil {
		link = issue.HTMLURL() + "#" + comment.HashTag()
	} else {
		link = issue.HTMLURL()
	}

	mailMeta := map[string]interface{}{
		"FallbackSubject": fallback,
		"Body":            body,
		"Link":            link,
		"Issue":           issue,
		"Comment":         comment,
		"IsPull":          issue.IsPull,
		"User":            issue.Repo.MustOwner().Name,
		"Repo":            issue.Repo.FullName(),
		"Doer":            doer,
		"Action":          actionType,
		"IsMention":       fromMention,
		"SubjectPrefix":   prefix,
	}

	tplBody := actionToTemplate(issue, actionType)

	var mailSubject bytes.Buffer
	if err := subjectTemplates.ExecuteTemplate(&mailSubject, string(tplBody), mailMeta); err == nil {
		subject = sanitizeSubject(mailSubject.String())
	} else {
		log.Error("ExecuteTemplate [%s]: %v", string(tplBody)+"/subject", err)
	}

	if subject == "" {
		subject = fallback
	}
	mailMeta["Subject"] = subject

	var mailBody bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplBody), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplBody)+"/body", err)
	}

	msg := NewMessageFrom(tos, doer.DisplayName(), setting.MailService.FromEmail, subject, mailBody.String())
	msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)

	// Set Message-ID on first message so replies know what to reference
	if comment == nil {
		msg.SetHeader("Message-ID", "<"+issue.ReplyReference()+">")
	} else {
		msg.SetHeader("In-Reply-To", "<"+issue.ReplyReference()+">")
		msg.SetHeader("References", "<"+issue.ReplyReference()+">")
	}

	return msg
}

func sanitizeSubject(subject string) string {
	runes := []rune(strings.TrimSpace(subjectRemoveSpaces.ReplaceAllLiteralString(subject, " ")))
	if len(runes) > mailMaxSubjectRunes {
		runes = runes[:mailMaxSubjectRunes]
	}
	// Encode non-ASCII characters
	return mime.QEncoding.Encode("utf-8", string(runes))
}

// SendIssueCommentMail composes and sends issue comment emails to target receivers.
func SendIssueCommentMail(issue *models.Issue, doer *models.User, actionType models.ActionType, content string, comment *models.Comment, tos []string) {
	if len(tos) == 0 {
		return
	}

	SendAsync(composeIssueCommentMessage(issue, doer, actionType, false, content, comment, tos, "issue comment"))
}

// SendIssueMentionMail composes and sends issue mention emails to target receivers.
func SendIssueMentionMail(issue *models.Issue, doer *models.User, actionType models.ActionType, content string, comment *models.Comment, tos []string) {
	if len(tos) == 0 {
		return
	}
	SendAsync(composeIssueCommentMessage(issue, doer, actionType, true, content, comment, tos, "issue mention"))
}

func actionToTemplate(issue *models.Issue, actionType models.ActionType) base.TplName {
	var name base.TplName
	switch actionType {
	case models.ActionCreateIssue:
		name = mailNewIssue
	case models.ActionCreatePullRequest:
		name = mailNewPullRequest
	case models.ActionCommentIssue:
		if issue.IsPull {
			name = mailCommentPullRequest
		} else {
			name = mailCommentIssue
		}
	case models.ActionCloseIssue:
		name = mailCloseIssue
	case models.ActionReopenIssue:
		name = mailReopenIssue
	case models.ActionClosePullRequest:
		name = mailClosePullRequest
	case models.ActionReopenPullRequest:
		name = mailReopenPullRequest
	case models.ActionMergePullRequest:
		name = mailMergePullRequest
	}
	if name != "" && bodyTemplates.Lookup(string(name)) != nil {
		return name
	}
	if issue.IsPull {
		return mailPullRequestDefault
	}
	return mailIssueDefault
}
