// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToTimelineCommentConfidentialRef(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	publicIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

	confidential := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9100, PosterID: poster.ID, Poster: poster, Title: "hidden zebrafinch", IsConfidential: true}
	require.NoError(t, db.Insert(t.Context(), confidential))
	refComment := &issues_model.Comment{Type: issues_model.CommentTypeIssueRef, IssueID: publicIssue.ID, Issue: publicIssue, PosterID: poster.ID, Poster: poster, RefRepoID: repo.ID, RefIssueID: confidential.ID}
	require.NoError(t, db.Insert(t.Context(), refComment))

	hidden := ToTimelineComment(t.Context(), repo, refComment, nil)
	require.NotNil(t, hidden)
	assert.Equal(t, ConfidentialRefPlaceholder, hidden.Body)
	assert.Nil(t, hidden.RefIssue)
	assert.Nil(t, hidden.RefComment)
	assert.Empty(t, hidden.RefCommitSHA)
	assert.Empty(t, hidden.RefAction)
	assert.Equal(t, poster.ID, hidden.Poster.ID)
	assert.Equal(t, refComment.CreatedUnix.AsTime(), hidden.Created)

	allowed := ToTimelineComment(t.Context(), repo, refComment, owner)
	require.NotNil(t, allowed)
	assert.NotEqual(t, ConfidentialRefPlaceholder, allowed.Body)
	require.NotNil(t, allowed.RefIssue)
	assert.Equal(t, confidential.Title, allowed.RefIssue.Title)
}

func TestToTimelineCommentConfidentialDependency(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	publicIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

	confidential := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9101, PosterID: poster.ID, Poster: poster, Title: "hidden nightjar", IsConfidential: true}
	require.NoError(t, db.Insert(t.Context(), confidential))

	for _, commentType := range []issues_model.CommentType{issues_model.CommentTypeAddDependency, issues_model.CommentTypeRemoveDependency} {
		comment := &issues_model.Comment{Type: commentType, IssueID: publicIssue.ID, Issue: publicIssue, PosterID: poster.ID, Poster: poster, DependentIssueID: confidential.ID, DependentIssue: confidential}
		require.NoError(t, db.Insert(t.Context(), comment))

		hidden := ToTimelineComment(t.Context(), repo, comment, nil)
		require.NotNil(t, hidden)
		assert.Nil(t, hidden.DependentIssue)

		allowed := ToTimelineComment(t.Context(), repo, comment, owner)
		require.NotNil(t, allowed)
		require.NotNil(t, allowed.DependentIssue)
		assert.Equal(t, confidential.Title, allowed.DependentIssue.Title)
	}
}
