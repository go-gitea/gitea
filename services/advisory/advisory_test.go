// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"testing"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStateOptions(t *testing.T) {
	triage := &advisory_model.Advisory{ID: 1, RepoID: 1, State: advisory_model.StateTriage}
	original := &advisory_model.Advisory{ID: 2, RepoID: 1}
	for _, c := range []struct {
		opts  StateOptions
		valid bool
	}{
		{StateOptions{State: advisory_model.StateDraft}, true},
		{StateOptions{State: advisory_model.StatePublished}, false},
		{StateOptions{State: advisory_model.StateDraft, CloseReason: advisory_model.CloseReasonInvalid}, false},
		{StateOptions{State: advisory_model.StateClosed}, false},
		{StateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonInvalid}, true},
		{StateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonInvalid, DuplicateOf: original}, false},
		{StateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate}, false},
		{StateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: original}, true},
		{StateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: triage}, false},
	} {
		assert.Equal(t, c.valid, validateStateOptions(triage, c.opts) == nil, "%+v", c.opts)
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
	assert.ErrorIs(t, AddCollaborator(ctx, admin, report, user(5), nil), user_model.ErrBlockedUser)

	// published credits are public, so private users cannot be credited
	opts := ContentFromAdvisory(report)
	opts.Credits = []*advisory_model.Credit{{UserID: 31, Type: "finder"}}
	assert.ErrorIs(t, UpdateAdvisory(ctx, admin, report, opts, true), util.ErrInvalidArgument)

	// adding a read-only collaborator again grants write access
	_, err := advisory_model.AddCollaborator(ctx, report.ID, 8, 0, true)
	require.NoError(t, err)
	require.NoError(t, AddCollaborator(ctx, admin, report, user(8), nil))
	perms, err := advisory_model.Viewer{Doer: user(8)}.Permissions(ctx, report)
	require.NoError(t, err)
	assert.True(t, perms.CanEdit)

	// teams need access to a private repository
	draft := newAdvisory(3, &advisory_model.Advisory{State: advisory_model.StateDraft, ReporterID: 2})
	team := func(id int64) *organization.Team {
		return unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: id})
	}
	assert.ErrorIs(t, AddCollaborator(ctx, admin, draft, nil, team(7)), util.ErrInvalidArgument)
	require.NoError(t, AddCollaborator(ctx, admin, draft, nil, team(2)))

	// an original cannot be deleted while duplicates refer to it
	duplicate := newAdvisory(3, &advisory_model.Advisory{State: advisory_model.StateClosed, ReporterID: 2, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOfID: draft.ID})
	assert.ErrorIs(t, DeleteAdvisory(ctx, admin, draft), util.ErrInvalidArgument)
	require.NoError(t, DeleteAdvisory(ctx, admin, duplicate))
	require.NoError(t, DeleteAdvisory(ctx, admin, draft))
}
