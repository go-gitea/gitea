// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models"
)

// SendRepoTransferNotifyMail triggers a notification e-mail when a pending repository transfer was created
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

	subject := fmt.Sprintf("%s would like to transfer \"%s\" to %s", doer.DisplayName(), repo.FullName(), destination)
	data := map[string]interface{}{
		"Doer":    doer,
		"User":    repo.Owner,
		"Repo":    repo.FullName(),
		"Link":    repo.HTMLURL(),
		"Subject": subject,

		"Destination": destination,
	}

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailRepoTransferNotify), data); err != nil {
		return err
	}

	msg := NewMessage(emails, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, repository pending transfer notification", newOwner.ID)

	SendAsync(msg)
	return nil
}
