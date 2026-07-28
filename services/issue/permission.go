// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
)

// CanEditIssueOrPullMeta reports whether doer may edit the title or body of an
// issue or pull request.
//
// Allowed when the user is the poster, has write access to issues/pulls on the
// base repository, or (for pull requests) has write access to the head
// repository — so collaborators on a fork can refine the PR they share with
// the author (see #36860).
func CanEditIssueOrPullMeta(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, basePerm access_model.Permission) (bool, error) {
	if doer == nil {
		return false, nil
	}
	if issue.IsPoster(doer.ID) {
		return true, nil
	}
	if basePerm.CanWriteIssuesOrPulls(issue.IsPull) {
		return true, nil
	}
	if !issue.IsPull {
		return false, nil
	}
	return canWriteToPullHeadRepo(ctx, doer, issue)
}

func canWriteToPullHeadRepo(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) (bool, error) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		return false, err
	}
	pr := issue.PullRequest
	if pr == nil {
		return false, nil
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return false, err
	}
	if pr.HeadRepo == nil {
		return false, nil
	}
	// Same-repo PRs: head write is already covered by base write above when the
	// user has PR write on the only repo; still check head for completeness.
	headPerm, err := access_model.GetDoerRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		return false, err
	}
	return headPerm.CanWrite(unit.TypeCode), nil
}
