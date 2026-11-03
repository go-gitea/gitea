// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
)

var (
	ErrConfidentialPullRequest      = util.ErrorWrap(util.ErrUnprocessableContent, "Pull requests cannot be confidential.")
	ErrConfidentialPermissionDenied = util.ErrorWrap(util.ErrPermissionDenied, "You do not have permission to change the confidential setting of this issue.")
)

// ChangeConfidential makes the issue confidential or public. The caller has already established that doer may view the issue.
func ChangeConfidential(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, perm access_model.Permission, confidential bool) error {
	if issue.IsConfidential == confidential {
		return nil
	}
	if issue.IsPull {
		return ErrConfidentialPullRequest
	}
	if !perm.CanWrite(unit.TypeIssues) {
		return ErrConfidentialPermissionDenied
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("ChangeConfidential: %w", err)
	}
	comment, err := issues_model.SetIssueConfidential(ctx, &issues_model.IssueConfidentialOptions{Doer: doer, Issue: issue}, confidential)
	if err != nil {
		return fmt.Errorf("ChangeConfidential: %w", err)
	}
	if comment != nil {
		notify_service.IssueChangeConfidential(ctx, doer, issue)
	}
	return nil
}
