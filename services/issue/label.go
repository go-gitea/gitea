// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
)

// ClearLabels clears all of an issue's labels
func ClearLabels(ctx context.Context, issue *issues_model.Issue, doer *user_model.User) error {
	if err := issues_model.ClearIssueLabels(ctx, issue, doer); err != nil {
		return err
	}

	notify_service.IssueClearLabels(ctx, doer, issue)

	return nil
}

// AddLabel adds a new label to the issue.
func AddLabel(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, label *issues_model.Label) error {
	if err := issues_model.NewIssueLabel(ctx, issue, label, doer); err != nil {
		return err
	}

	notify_service.IssueChangeLabels(ctx, doer, issue, []*issues_model.Label{label}, nil)
	return nil
}

// AddLabels adds a list of new labels to the issue.
func AddLabels(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, labels []*issues_model.Label) error {
	if err := issues_model.NewIssueLabels(ctx, issue, labels, doer); err != nil {
		return err
	}

	notify_service.IssueChangeLabels(ctx, doer, issue, labels, nil)
	return nil
}

// RemoveLabel removes a label from issue by given ID.
func RemoveLabel(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, label *issues_model.Label) error {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := issue.LoadRepo(dbCtx); err != nil {
		return err
	}

	perm, err := access_model.GetUserRepoPermission(dbCtx, issue.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(issue.IsPull) {
		if label.OrgID > 0 {
			return issues_model.ErrOrgLabelNotExist{}
		}
		return issues_model.ErrRepoLabelNotExist{}
	}

	if err := issues_model.DeleteIssueLabel(dbCtx, issue, label, doer); err != nil {
		return err
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	notify_service.IssueChangeLabels(ctx, doer, issue, nil, []*issues_model.Label{label})
	return nil
}

// ReplaceLabels removes all current labels and add new labels to the issue.
func ReplaceLabels(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, labels []*issues_model.Label) error {
	old, err := issues_model.GetLabelsByIssueID(ctx, issue.ID)
	if err != nil {
		return err
	}

	if err := issues_model.ReplaceIssueLabels(ctx, issue, labels, doer); err != nil {
		return err
	}

	notify_service.IssueChangeLabels(ctx, doer, issue, labels, old)
	return nil
}
