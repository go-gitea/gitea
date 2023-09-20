// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package mailer

import (
	"bytes"
	"context"
	"strconv"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
)

const (
	tplNewUserMail base.TplName = "admin_new_user"
)

var sa = SendAsyncs

// MailNewUser sends notification emails on new user registrations to all admins
func MailNewUser(ctx context.Context, u *user_model.User) {
	if !setting.Admin.SendNotificationEmailOnNewUser {
		return
	}

	if setting.MailService == nil {
		// No mail service configured
		return
	}

	recipients, err := user_model.GetAllAdmins(ctx)
	if err != nil {
		log.Error("user_model.GetAllAdmins: %v", err)
		return
	}

	langMap := make(map[string][]string)
	for _, r := range recipients {
		langMap[r.Language] = append(langMap[r.Language], r.Email)
	}

	for lang, tos := range langMap {
		mailNewUser(ctx, u, lang, tos)
	}
}

func mailNewUser(ctx context.Context, u *user_model.User, lang string, tos []string) {
	locale := translation.NewLocale(lang)

	subject := locale.Tr("mail.admin.new_user.subject", u.Name)
	manageUserURL := setting.AppSubURL + "/admin/users/" + strconv.FormatInt(u.ID, 10)
	body := locale.Tr("mail.admin.new_user.text", manageUserURL)
	mailMeta := map[string]any{
		"NewUser":  u,
		"Subject":  subject,
		"Body":     body,
		"Language": locale.Language(),
		"locale":   locale,
		"Str2html": templates.Str2html,
	}

	var mailBody bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplNewUserMail), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplNewUserMail)+"/body", err)
		return
	}

	msgs := make([]*Message, 0, len(tos))
	for _, to := range tos {
		msg := NewMessage(to, subject, mailBody.String())
		msg.Info = subject
		msgs = append(msgs, msg)
	}
	sa(msgs)
}
