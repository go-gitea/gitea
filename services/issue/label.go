// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	user_model "gitea.dev/models/user"
	notify_service "gitea.dev/services/notify"
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
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		perm, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, doer)
		if err != nil {
			return err
		}
		if !perm.CanWriteIssuesOrPulls(issue.IsPull) {
			if label.OrgID > 0 {
				return issues_model.ErrOrgLabelNotExist{}
			}
			return issues_model.ErrRepoLabelNotExist{}
		}

		return issues_model.DeleteIssueLabel(ctx, issue, label, doer)
	}); err != nil {
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

// AddRemoveLabels adds and removes labels in one transaction, notifying only the labels that actually changed
func AddRemoveLabels(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, toAdd, toRemove []*issues_model.Label) error {
	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}
	if err := issue.LoadLabels(ctx); err != nil {
		return err
	}
	hasLabel := func(label *issues_model.Label) bool {
		return slices.ContainsFunc(issue.Labels, func(l *issues_model.Label) bool { return l.ID == label.ID })
	}
	toAdd = slices.DeleteFunc(slices.Clone(toAdd), func(l *issues_model.Label) bool {
		return hasLabel(l) || (l.RepoID != issue.RepoID && l.OrgID != issue.Repo.OwnerID)
	})
	toRemove = slices.DeleteFunc(slices.Clone(toRemove), func(l *issues_model.Label) bool { return !hasLabel(l) })
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if len(toAdd) > 0 {
			if err := issues_model.NewIssueLabels(ctx, issue, toAdd, doer); err != nil {
				return err
			}
		}
		for _, label := range toRemove {
			if err := issues_model.DeleteIssueLabel(ctx, issue, label, doer); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	notify_service.IssueChangeLabels(ctx, doer, issue, toAdd, toRemove)
	return nil
}
