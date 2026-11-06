// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"testing"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStateOptions(t *testing.T) {
	triage := &advisory_model.Advisory{ID: 1, RepoID: 1, State: advisory_model.StateTriage}
	original := &advisory_model.Advisory{ID: 2, RepoID: 1}
	duplicate := &advisory_model.Advisory{ID: 3, RepoID: 1, DuplicateOfID: 1}
	for opts, valid := range map[StateOptions]bool{
		{State: advisory_model.StateDraft}:                                                                            true,
		{State: advisory_model.StatePublished}:                                                                        false,
		{State: advisory_model.StateDraft, CloseReason: advisory_model.CloseReasonInvalid}:                            false,
		{State: advisory_model.StateClosed}:                                                                           false,
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonInvalid}:                           true,
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonInvalid, DuplicateOf: original}:    false,
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate}:                         false,
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: original}:  true,
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: triage}:    false,
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: duplicate}: false,
	} {
		assert.Equal(t, valid, validateStateOptions(triage, opts) == nil, "%+v", opts)
	}
}

func TestCollaboratorAndCreditRules(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	user := func(id int64) *user_model.User { return unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id}) }
	admin := user(2)
	require.NoError(t, db.Insert(ctx, []*repo_model.RepoUnit{{RepoID: 1, Type: unit.TypeSecurityAdvisories}, {RepoID: 3, Type: unit.TypeSecurityAdvisories}}))
	newAdvisory := func(repoID int64, a *advisory_model.Advisory) *advisory_model.Advisory {
		a.RepoID, a.Summary, a.Description = repoID, "s", "d"
		require.NoError(t, advisory_model.CreateAdvisory(ctx, a))
		require.NoError(t, a.LoadAttributes(ctx))
		return a
	}
	report := newAdvisory(1, &advisory_model.Advisory{State: advisory_model.StateTriage, IsReport: true, ReporterID: 4})

	// the reporter's blocks count like the owner's because a report is their content
	require.NoError(t, db.Insert(ctx, &user_model.Blocking{BlockerID: 4, BlockeeID: 5}))
	assert.ErrorIs(t, AddCollaborator(ctx, admin, report, user(5).Name, false), user_model.ErrBlockedUser)

	// published credits are public, so private users cannot be credited
	adminPerms := advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true, CanManage: true}
	opts := EditOptions{Credits: optional.Some([]*advisory_model.Credit{{UserID: 31, Type: "finder"}})}
	assert.ErrorIs(t, EditAdvisory(ctx, admin, report, adminPerms, opts), util.ErrInvalidArgument)

	// adding a read-only collaborator again grants write access
	_, err := advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: report.ID, UserID: 8, ReadOnly: true})
	require.NoError(t, err)
	require.NoError(t, AddCollaborator(ctx, admin, report, user(8).Name, false))
	perms, err := advisory_model.Viewer{Doer: user(8)}.Permissions(ctx, report)
	require.NoError(t, err)
	assert.True(t, perms.CanEdit)

	// teams need access to a private repository
	draft := newAdvisory(3, &advisory_model.Advisory{State: advisory_model.StateDraft, ReporterID: 2})
	assert.ErrorIs(t, AddCollaborator(ctx, admin, draft, "test_team", true), util.ErrInvalidArgument)
	require.NoError(t, AddCollaborator(ctx, admin, draft, "team1", true))

	// an original cannot be deleted while duplicates refer to it
	duplicate := newAdvisory(3, &advisory_model.Advisory{State: advisory_model.StateClosed, ReporterID: 2, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOfID: draft.ID})
	assert.ErrorIs(t, DeleteAdvisory(ctx, admin, draft), util.ErrInvalidArgument)
	require.NoError(t, DeleteAdvisory(ctx, admin, duplicate))
	require.NoError(t, DeleteAdvisory(ctx, admin, draft))

	// the reporter of duplicates follows the original read-only until no report is a duplicate of it anymore
	original := newAdvisory(1, &advisory_model.Advisory{State: advisory_model.StateDraft, ReporterID: 2})
	reporterPerms := func() advisory_model.Permissions {
		perms, err := advisory_model.Viewer{Doer: user(8)}.Permissions(ctx, original)
		require.NoError(t, err)
		return perms
	}
	closeAsDuplicate := StateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: original}
	first := newAdvisory(1, &advisory_model.Advisory{State: advisory_model.StateTriage, IsReport: true, ReporterID: 8})
	second := newAdvisory(1, &advisory_model.Advisory{State: advisory_model.StateTriage, IsReport: true, ReporterID: 8})
	require.NoError(t, ChangeState(ctx, admin, first, closeAsDuplicate))
	require.NoError(t, ChangeState(ctx, admin, second, closeAsDuplicate))
	assert.True(t, reporterPerms().CanView)

	// neither an unset list nor naming them again upgrades read-only collaborators
	require.NoError(t, EditAdvisory(ctx, admin, original, adminPerms, EditOptions{CollaboratorTeams: optional.Some([]string{})}))
	assert.False(t, reporterPerms().CanEdit)
	require.NoError(t, EditAdvisory(ctx, admin, original, adminPerms, EditOptions{CollaboratorUsers: optional.Some([]string{user(8).Name})}))
	assert.False(t, reporterPerms().CanEdit)

	require.NoError(t, ChangeState(ctx, admin, first, StateOptions{State: advisory_model.StateDraft}))
	assert.True(t, reporterPerms().CanView, "the other report is still a duplicate")
	require.NoError(t, DeleteAdvisory(ctx, admin, second))
	assert.False(t, reporterPerms().CanView, "deleting the last duplicate revokes the access")

	// a failed revocation rolls back the state, so that reopening can be retried
	third := newAdvisory(1, &advisory_model.Advisory{State: advisory_model.StateTriage, IsReport: true, ReporterID: 8})
	require.NoError(t, ChangeState(ctx, admin, third, closeAsDuplicate))
	renameCollaboratorTable := func(from, to string) {
		_, err := db.GetEngine(ctx).Exec("ALTER TABLE " + from + " RENAME TO " + to)
		require.NoError(t, err)
	}
	renameCollaboratorTable("security_advisory_collaborator", "security_advisory_collaborator_tmp")
	reopenErr := ChangeState(ctx, admin, third, StateOptions{State: advisory_model.StateDraft})
	renameCollaboratorTable("security_advisory_collaborator_tmp", "security_advisory_collaborator")
	require.Error(t, reopenErr)
	third = unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{ID: third.ID, State: advisory_model.StateClosed, DuplicateOfID: original.ID})
	require.NoError(t, third.LoadAttributes(ctx))
	require.NoError(t, ChangeState(ctx, admin, third, StateOptions{State: advisory_model.StateDraft}))
	assert.False(t, reporterPerms().CanView)
}
