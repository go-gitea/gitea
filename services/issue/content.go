// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
)

// ChangeContent changes issue content, as the given user.
func ChangeContent(issue *issues_model.Issue, doer *user_model.User, content string) (err error) {
	oldContent := issue.Content

	if err := issues_model.ChangeIssueContent(issue, doer, content); err != nil {
		return err
	}

	notify_service.IssueChangeContent(db.DefaultContext, doer, issue, oldContent)

	return nil
}
