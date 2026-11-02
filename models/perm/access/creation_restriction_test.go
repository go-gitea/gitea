// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"testing"

	"gitea.dev/models/db"
	perm_model "gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCreationRestriction(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}) // public repo, owner id=2
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	outsider := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	require.True(t, admin.IsAdmin)

	require.NoError(t, db.TruncateBeans(ctx, &repo_model.Collaboration{}, &Access{}))

	t.Run("ReaderModeAllowsAnyoneIncludingNilDoer", func(t *testing.T) {
		result, err := CheckCreationRestriction(ctx, repo1, nil, unit.TypeIssues, repo_model.CreationRestrictionReader)
		require.NoError(t, err)
		assert.Equal(t, CreationAllowed, result)

		result, err = CheckCreationRestriction(ctx, repo1, outsider, unit.TypeIssues, repo_model.CreationRestrictionReader)
		require.NoError(t, err)
		assert.Equal(t, CreationAllowed, result)
	})

	t.Run("NilDoerIsDeniedForMemberAndWriterModes", func(t *testing.T) {
		result, err := CheckCreationRestriction(ctx, repo1, nil, unit.TypeIssues, repo_model.CreationRestrictionMember)
		require.NoError(t, err)
		assert.Equal(t, CreationDeniedNotCollaborator, result)

		result, err = CheckCreationRestriction(ctx, repo1, nil, unit.TypeIssues, repo_model.CreationRestrictionWriter)
		require.NoError(t, err)
		assert.Equal(t, CreationDeniedNotCollaborator, result)
	})

	t.Run("AdminAlwaysAllowed", func(t *testing.T) {
		result, err := CheckCreationRestriction(ctx, repo1, admin, unit.TypeIssues, repo_model.CreationRestrictionWriter)
		require.NoError(t, err)
		assert.Equal(t, CreationAllowed, result)
	})

	t.Run("OwnerAlwaysAllowed", func(t *testing.T) {
		result, err := CheckCreationRestriction(ctx, repo1, owner, unit.TypeIssues, repo_model.CreationRestrictionWriter)
		require.NoError(t, err)
		assert.Equal(t, CreationAllowed, result)
	})

	t.Run("MemberModeDeniesNonCollaborator", func(t *testing.T) {
		result, err := CheckCreationRestriction(ctx, repo1, outsider, unit.TypeIssues, repo_model.CreationRestrictionMember)
		require.NoError(t, err)
		assert.Equal(t, CreationDeniedNotCollaborator, result)
	})

	t.Run("MemberModeAllowsReadOnlyCollaborator", func(t *testing.T) {
		require.NoError(t, db.Insert(ctx, &repo_model.Collaboration{RepoID: repo1.ID, UserID: outsider.ID, Mode: perm_model.AccessModeRead}))
		require.NoError(t, db.Insert(ctx, &Access{RepoID: repo1.ID, UserID: outsider.ID, Mode: perm_model.AccessModeRead}))
		defer func() {
			require.NoError(t, db.TruncateBeans(ctx, &repo_model.Collaboration{}, &Access{}))
		}()

		result, err := CheckCreationRestriction(ctx, repo1, outsider, unit.TypeIssues, repo_model.CreationRestrictionMember)
		require.NoError(t, err)
		assert.Equal(t, CreationAllowed, result)
	})

	t.Run("WriterModeDeniesReadOnlyCollaborator", func(t *testing.T) {
		require.NoError(t, db.Insert(ctx, &repo_model.Collaboration{RepoID: repo1.ID, UserID: outsider.ID, Mode: perm_model.AccessModeRead}))
		require.NoError(t, db.Insert(ctx, &Access{RepoID: repo1.ID, UserID: outsider.ID, Mode: perm_model.AccessModeRead}))
		defer func() {
			require.NoError(t, db.TruncateBeans(ctx, &repo_model.Collaboration{}, &Access{}))
		}()

		result, err := CheckCreationRestriction(ctx, repo1, outsider, unit.TypeIssues, repo_model.CreationRestrictionWriter)
		require.NoError(t, err)
		assert.Equal(t, CreationDeniedNeedsWriteAccess, result)
	})

	t.Run("WriterModeAllowsWriteCollaborator", func(t *testing.T) {
		require.NoError(t, db.Insert(ctx, &repo_model.Collaboration{RepoID: repo1.ID, UserID: outsider.ID, Mode: perm_model.AccessModeWrite}))
		require.NoError(t, db.Insert(ctx, &Access{RepoID: repo1.ID, UserID: outsider.ID, Mode: perm_model.AccessModeWrite}))
		defer func() {
			require.NoError(t, db.TruncateBeans(ctx, &repo_model.Collaboration{}, &Access{}))
		}()

		result, err := CheckCreationRestriction(ctx, repo1, outsider, unit.TypeIssues, repo_model.CreationRestrictionWriter)
		require.NoError(t, err)
		assert.Equal(t, CreationAllowed, result)
	})
}
