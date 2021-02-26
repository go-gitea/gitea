// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models"
)

// SendRepoTransferNotifyMail triggers a notification e-mail when a repository transfer is initiated
func SendRepoTransferNotifyMail(doer, newOwner *models.User, repo *models.Repository) error {
	var (
		emails      []string
		destination string
		content     bytes.Buffer
	)

	if newOwner.IsOrganization() {
		users, err := models.GetUsersWhoCanCreateOrgRepo(newOwner.ID)
		if err != nil {
			return err
		}

		for i := range users {
			emails = append(emails, users[i].Email)
		}
		destination = newOwner.DisplayName()
	} else {
		emails = []string{newOwner.Email}
		destination = "you"
	}

	data := map[string]interface{}{
		"Doer":    doer,
		"User":    repo.Owner,
		"Repo":    repo.FullName(),
		"Link":    repo.HTMLURL(),
		"Subject": fmt.Sprintf("%s like to transfer \"%s\" to %s", doer.DisplayName(), repo.FullName(), destination),

		"Destination": destination,
	}

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailRepoTransferNotify), data); err != nil {
		return err
	}

	// msg := NewMessage([]string{email}, locale.Tr("mail.repo_transfer_notify"), content.String())
	msg := NewMessage(emails, "mail.repo_transfer_notify", content.String())
	msg.Info = fmt.Sprintf("UID: %d, repository transfer notification", newOwner.ID)

	SendAsync(msg)
	return nil
}
