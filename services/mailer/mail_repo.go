// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"

	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

const (
	mailNotifyCollaborator templates.TplName = "repo/collaborator"
	mailRepoTransferNotify templates.TplName = "repo/transfer"
)

// SendRepoTransferNotifyMail triggers a notification e-mail when a pending repository transfer was created
func SendRepoTransferNotifyMail(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	if newOwner.IsOrganization() {
		users, err := organization.GetUsersWhoCanCreateOrgRepo(ctx, newOwner.ID)
		if err != nil {
			return err
		}

		langMap := make(map[string][]*user_model.User)
		for _, user := range users {
			if !user.IsActive {
				// don't send emails to inactive users
				continue
			}
			langMap[user.Language] = append(langMap[user.Language], user)
		}

		for lang, tos := range langMap {
			if err := sendRepoTransferNotifyMailPerLang(lang, newOwner, doer, tos, repo); err != nil {
				return err
			}
		}

		return nil
	}

	return sendRepoTransferNotifyMailPerLang(newOwner.Language, newOwner, doer, []*user_model.User{newOwner}, repo)
}

// sendRepoTransferNotifyMail triggers a notification e-mail when a pending repository transfer was created for each language
func sendRepoTransferNotifyMailPerLang(lang string, newOwner, doer *user_model.User, emailTos []*user_model.User, repo *repo_model.Repository) error {
	var (
		locale  = translation.NewLocale(lang)
		content bytes.Buffer
	)

	destination := locale.TrString("mail.repo.transfer.to_you")
	subject := locale.TrString("mail.repo.transfer.subject_to_you", doer.DisplayName(), repo.FullName())
	if newOwner.IsOrganization() {
		destination = newOwner.DisplayName()
		subject = locale.TrString("mail.repo.transfer.subject_to", doer.DisplayName(), repo.FullName(), destination)
	}

	data := map[string]any{
		"locale":      locale,
		"Doer":        doer,
		"User":        repo.Owner,
		"Repo":        repo.FullName(),
		"Link":        repo.HTMLURL(),
		"Subject":     subject,
		"Language":    locale.Language(),
		"Destination": destination,
	}

	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&content, string(mailRepoTransferNotify), data); err != nil {
		return err
	}

	for _, to := range emailTos {
		msg := sender_service.NewMessageFrom(to.EmailTo(), fromDisplayName(doer), setting.MailService.FromEmail, subject, content.String())
		msg.Info = fmt.Sprintf("UID: %d, repository pending transfer notification", newOwner.ID)

		SendAsync(msg)
	}

	return nil
}

// SendCollaboratorMail sends mail notification to new collaborator.
func SendCollaboratorMail(u, doer *user_model.User, repo *repo_model.Repository) {
	if setting.MailService == nil || !u.IsActive {
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

	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&content, string(mailNotifyCollaborator), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := sender_service.NewMessage(u.EmailTo(), subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, add collaborator", u.ID)

	SendAsync(msg)
}
