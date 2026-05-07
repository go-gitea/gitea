// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// GetCommitCommentIssue returns the synthetic carrier Issue for a (repo, sha)
// pair, or `nil, nil` if no comments have been posted on that commit yet.
//
// The carrier Issue is intentionally hidden from the regular issues UI by the
// `issue.commit_sha = ''` filter in applyConditions; callers that want to
// surface it must look it up via this helper (or by ID).
func GetCommitCommentIssue(ctx context.Context, repoID int64, commitSHA string) (*Issue, error) {
	if commitSHA == "" {
		return nil, errors.New("empty commit SHA")
	}
	issue := new(Issue)
	has, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repoID, "commit_sha": commitSHA}).
		Get(issue)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return issue, nil
}

// GetOrCreateCommitCommentIssue returns the synthetic carrier Issue for a
// (repo, sha) pair, creating one on the fly the first time it is requested.
//
// Concurrent first-comment-on-the-same-commit requests can race: two callers
// may both observe "no carrier exists" and both attempt to insert. The unique
// (repo_id, index) constraint on issue makes the loser's insert fail; we
// recover from that by re-reading the row that the winner inserted.
//
// The carrier issue's title / content are set to a fixed sentinel ("Comments
// on commit <sha>") so admin tools that join through Issue have something
// human-readable to display, but they are *not* meant to be edited via the
// normal issue UI — see services/commit/comment.go for the entry points.
//
// Counters on Repository.NumIssues / NumClosedIssues are deliberately *not*
// incremented: this isn't a "real" issue. The `commit_sha != ''` filter in
// applyConditions hides the row from every list / count query in the codebase.
func GetOrCreateCommitCommentIssue(ctx context.Context, repo *repo_model.Repository, commitSHA string, doer *user_model.User) (*Issue, error) {
	if commitSHA == "" {
		return nil, errors.New("empty commit SHA")
	}
	if repo == nil {
		return nil, errors.New("nil repo")
	}
	if doer == nil {
		return nil, errors.New("nil doer")
	}

	if existing, err := GetCommitCommentIssue(ctx, repo.ID, commitSHA); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	var carrier *Issue
	err := db.WithTx(ctx, func(ctx context.Context) error {
		// Re-check inside the transaction in case another writer beat us.
		if existing, err := GetCommitCommentIssue(ctx, repo.ID, commitSHA); err != nil {
			return err
		} else if existing != nil {
			carrier = existing
			return nil
		}

		idx, err := db.GetNextResourceIndex(ctx, "issue_index", repo.ID)
		if err != nil {
			return fmt.Errorf("generate issue index for commit comment carrier: %w", err)
		}

		carrier = &Issue{
			RepoID:    repo.ID,
			Repo:      repo,
			Index:     idx,
			PosterID:  doer.ID,
			Poster:    doer,
			Title:     fmt.Sprintf("Comments on commit %s", commitSHA),
			Content:   "",
			IsClosed:  false,
			IsPull:    false,
			CommitSHA: commitSHA,
		}

		if _, err := db.GetEngine(ctx).Insert(carrier); err != nil {
			return fmt.Errorf("insert commit comment carrier: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return carrier, nil
}
