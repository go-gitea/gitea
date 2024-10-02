// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

var ErrIssueAlreadyChanged = util.NewInvalidArgumentErrorf("the issue is already changed")

// ChangeContent changes issue content, as the given user.
func ChangeContent(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, content string, contentVersion int) error {
	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, doer, issue.PosterID, issue.Repo.OwnerID) {
		if isAdmin, _ := access_model.IsUserRepoAdmin(ctx, issue.Repo, doer); !isAdmin {
			return user_model.ErrBlockedUser
		}
	}

	oldContent := issue.Content

	hasContentHistory, err := issues_model.HasIssueContentHistory(ctx, issue.ID, 0)
	if err != nil {
		return fmt.Errorf("HasIssueContentHistory: %w", err)
	}
	if !hasContentHistory {
		if err = issues_model.SaveIssueContentHistory(ctx, issue.PosterID, issue.ID, 0,
			issue.CreatedUnix, issue.Content, true); err != nil {
			return fmt.Errorf("SaveIssueContentHistory: %w", err)
		}
	}

	issue.Content = content
	issue.ContentVersion = contentVersion + 1

	affected, err := db.GetEngine(ctx).ID(issue.ID).Cols("content", "content_version").Where("content_version = ?", contentVersion).Update(issue)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrIssueAlreadyChanged
	}

	if err = issues_model.SaveIssueContentHistory(ctx, doer.ID, issue.ID, 0,
		timeutil.TimeStampNow(), issue.Content, false); err != nil {
		return fmt.Errorf("SaveIssueContentHistory: %w", err)
	}

	if err = issue.AddCrossReferences(ctx, doer, true); err != nil {
		return fmt.Errorf("addCrossReferences: %w", err)
	}

	notify_service.IssueChangeContent(ctx, doer, issue, oldContent)

	return nil
}
