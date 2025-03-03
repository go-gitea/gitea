// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"html/template"
	"mime"
	"regexp"
	"strings"
	texttmpl "text/template"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

const mailMaxSubjectRunes = 256 // There's no actual limit for subject in RFC 5322

var (
	bodyTemplates       *template.Template
	subjectTemplates    *texttmpl.Template
	subjectRemoveSpaces = regexp.MustCompile(`[\s]+`)
)

// SendTestMail sends a test mail
func SendTestMail(email string) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}
	return sender_service.Send(sender, sender_service.NewMessage(email, "Gitea Test Email!", "Gitea Test Email!"))
}

func sanitizeSubject(subject string) string {
	runes := []rune(strings.TrimSpace(subjectRemoveSpaces.ReplaceAllLiteralString(subject, " ")))
	if len(runes) > mailMaxSubjectRunes {
		runes = runes[:mailMaxSubjectRunes]
	}
	// Encode non-ASCII characters
	return mime.QEncoding.Encode("utf-8", string(runes))
}

func fromDisplayName(u *user_model.User) string {
	if setting.MailService.FromDisplayNameFormatTemplate != nil {
		var ctx bytes.Buffer
		err := setting.MailService.FromDisplayNameFormatTemplate.Execute(&ctx, map[string]any{
			"DisplayName": u.DisplayName(),
			"AppName":     setting.AppName,
			"Domain":      setting.Domain,
		})
		if err == nil {
			return mime.QEncoding.Encode("utf-8", ctx.String())
		}
		log.Error("fromDisplayName: %w", err)
	}
	return u.GetCompleteName()
}
