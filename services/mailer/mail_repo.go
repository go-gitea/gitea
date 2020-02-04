// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// SendRepoTransferNotifyMail triggers a notification e-mail when a repository
// transfer is initiated
func SendRepoTransferNotifyMail(locale Locale, u *models.User, repo *models.Repository) {
	data := map[string]interface{}{
		"Subject":             locale.Tr("mail.repo_transfer_notify"),
		"RepoName":            repo.FullName(),
		"Link":                repo.HTMLURL(),
		"AcceptTransferLink":  repo.HTMLURL() + "/action/accept_transfer",
		"DeclineTransferLink": repo.HTMLURL() + "/action/decline_transfer",
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailRepoTransferNotify), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	var email = u.Email

	if u.IsOrganization() && u.Email == "" {
		t, err := u.GetOwnerTeam()
		if err != nil {
			log.Error("Could not retrieve owners team for organization", err)
			return
		}

		if err := t.GetMembers(&models.SearchMembersOptions{}); err != nil {
			log.Error("Could not retrieve members of the owners team", err)
			return
		}

		// Just use the email address of the first user
		email = t.Members[0].Email
	}

	msg := NewMessage([]string{email}, locale.Tr("mail.repo_transfer_notify"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, repository transfer notification", u.ID)

	SendAsync(msg)
}
