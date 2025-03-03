// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

const (
	mailAuthActivate       templates.TplName = "auth/activate"
	mailAuthActivateEmail  templates.TplName = "auth/activate_email"
	mailAuthResetPassword  templates.TplName = "auth/reset_passwd"
	mailAuthRegisterNotify templates.TplName = "auth/register_notify"
	mailNotifyCollaborator templates.TplName = "notify/collaborator"
)

// sendUserMail sends a mail to the user
func sendUserMail(language string, u *user_model.User, tpl templates.TplName, code, subject, info string) {
	locale := translation.NewLocale(language)
	data := map[string]any{
		"locale":            locale,
		"DisplayName":       u.DisplayName(),
		"ActiveCodeLives":   timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale),
		"ResetPwdCodeLives": timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, locale),
		"Code":              code,
		"Language":          locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(tpl), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := sender_service.NewMessage(u.EmailTo(), subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, %s", u.ID, info)

	SendAsync(msg)
}

// SendActivateAccountMail sends an activation mail to the user (new user registration)
func SendActivateAccountMail(locale translation.Locale, u *user_model.User) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}
	opts := &user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeActivateAccount}
	sendUserMail(locale.Language(), u, mailAuthActivate, user_model.GenerateUserTimeLimitCode(opts, u), locale.TrString("mail.activate_account"), "activate account")
}

// SendResetPasswordMail sends a password reset mail to the user
func SendResetPasswordMail(u *user_model.User) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}
	locale := translation.NewLocale(u.Language)
	opts := &user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeResetPassword}
	sendUserMail(u.Language, u, mailAuthResetPassword, user_model.GenerateUserTimeLimitCode(opts, u), locale.TrString("mail.reset_password"), "recover account")
}

// SendActivateEmailMail sends confirmation email to confirm new email address
func SendActivateEmailMail(u *user_model.User, email string) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}
	locale := translation.NewLocale(u.Language)
	opts := &user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeActivateEmail, NewEmail: email}
	data := map[string]any{
		"locale":          locale,
		"DisplayName":     u.DisplayName(),
		"ActiveCodeLives": timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale),
		"Code":            user_model.GenerateUserTimeLimitCode(opts, u),
		"Email":           email,
		"Language":        locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthActivateEmail), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := sender_service.NewMessage(email, locale.TrString("mail.activate_email"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, activate email", u.ID)

	SendAsync(msg)
}

// SendRegisterNotifyMail triggers a notify e-mail by admin created a account.
func SendRegisterNotifyMail(u *user_model.User) {
	if setting.MailService == nil || !u.IsActive {
		// No mail service configured OR user is inactive
		return
	}
	locale := translation.NewLocale(u.Language)

	data := map[string]any{
		"locale":      locale,
		"DisplayName": u.DisplayName(),
		"Username":    u.Name,
		"Language":    locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthRegisterNotify), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := sender_service.NewMessage(u.EmailTo(), locale.TrString("mail.register_notify", setting.AppName), content.String())
	msg.Info = fmt.Sprintf("UID: %d, registration notify", u.ID)

	SendAsync(msg)
}

// SendCollaboratorMail sends mail notification to new collaborator.
func SendCollaboratorMail(u, doer *user_model.User, repo *repo_model.Repository) {
	if setting.MailService == nil || !u.IsActive {
		// No mail service configured OR the user is inactive
		return
	}
	locale := translation.NewLocale(u.Language)
	repoName := repo.FullName()

	subject := locale.TrString("mail.repo.collaborator.added.subject", doer.DisplayName(), repoName)
	data := map[string]any{
		"locale":   locale,
		"Subject":  subject,
		"RepoName": repoName,
		"Link":     repo.HTMLURL(),
		"Language": locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailNotifyCollaborator), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := sender_service.NewMessage(u.EmailTo(), subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, add collaborator", u.ID)

	SendAsync(msg)
}
