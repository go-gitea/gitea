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
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStateOptions(t *testing.T) {
	triage := &advisory_model.Advisory{ID: 1, State: advisory_model.StateTriage}
	original := &advisory_model.Advisory{ID: 2}
	for _, opts := range []stateOptions{
		{State: advisory_model.StateClosed},
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonInvalid},
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: original},
	} {
		assert.NoError(t, validateStateOptions(triage, opts))
	}
	for _, opts := range []stateOptions{
		{State: advisory_model.StatePublished},
		{State: advisory_model.StateDraft, CloseReason: advisory_model.CloseReasonInvalid},
		{State: advisory_model.StateClosed, DuplicateOf: original},
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate},
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: triage},
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: &advisory_model.Advisory{ID: 3, DuplicateOfID: original.ID}},
		{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: original, ShareOriginal: true},
	} {
		assert.ErrorIs(t, validateStateOptions(triage, opts), util.ErrInvalidArgument, "%+v", opts)
	}
	closed := &advisory_model.Advisory{ID: 5, State: advisory_model.StateClosed, IsReport: true}
	assert.NoError(t, validateStateOptions(closed, stateOptions{State: advisory_model.StateTriage}))
	closed.ReportDescription = "accepted before"
	assert.ErrorIs(t, validateStateOptions(closed, stateOptions{State: advisory_model.StateTriage}), util.ErrInvalidArgument)
	report := &advisory_model.Advisory{ID: 4, State: advisory_model.StateTriage, IsReport: true}
	assert.NoError(t, validateStateOptions(report, stateOptions{State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOf: original, ShareOriginal: true}))
	assert.ErrorIs(t, validateStateOptions(report, stateOptions{State: advisory_model.StateClosed, ShareOriginal: true}), util.ErrInvalidArgument)
}

func TestAdvisorySeverityEdits(t *testing.T) {
	const vector40 = "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:N/VI:N/VA:N/SC:N/SI:N/SA:N"
	const vector31 = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	advisory := &advisory_model.Advisory{RepoID: 1, State: advisory_model.StateDraft, ReporterID: 2, Summary: "Summary", Description: "Details", Severity: advisory_model.SeverityHigh}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, advisory))
	edit := func(opts EditOptions) error {
		return EditAdvisory(ctx, &user_model.User{ID: 2}, advisory, advisory_model.Permissions{CanEdit: true, CanManage: true}, opts)
	}
	read := func() *advisory_model.Advisory {
		t.Helper()
		stored, err := advisory_model.GetAdvisoryByIdentifier(ctx, advisory.RepoID, advisory.Identifier)
		require.NoError(t, err)
		return stored
	}

	assert.ErrorIs(t, edit(EditOptions{CvssVector: optional.Some(vector40), Severity: optional.Some("high")}), util.ErrInvalidArgument)
	assert.Equal(t, advisory_model.SeverityHigh, advisory.Severity)
	require.NoError(t, edit(EditOptions{CvssVector: optional.Some(" " + vector40 + " ")}))
	assert.Equal(t, vector40, advisory.CvssVector)
	assert.Equal(t, cvss.Version40, advisory.CvssVersion())
	assert.Zero(t, advisory.CvssScore())
	assert.Equal(t, advisory_model.SeverityUnknown, advisory.Severity)

	assert.ErrorIs(t, edit(EditOptions{Severity: optional.Some("low")}), util.ErrInvalidArgument)
	require.NoError(t, edit(EditOptions{CvssVector: optional.Some(vector31)}))
	stored := read()
	assert.Equal(t, vector31, stored.CvssVector)
	assert.Equal(t, cvss.Version31, stored.CvssVersion())
	assert.InDelta(t, 9.8, stored.CvssScore(), 1e-9)
	assert.Equal(t, advisory_model.SeverityCritical, stored.Severity)

	require.NoError(t, edit(EditOptions{CvssVector: optional.Some("")}))
	stored = read()
	assert.Empty(t, stored.CvssVector)
	assert.Empty(t, stored.CvssVersion())
	assert.Zero(t, stored.CvssScore())
	assert.Equal(t, advisory_model.SeverityUnknown, stored.Severity)
	require.NoError(t, edit(EditOptions{Severity: optional.Some("low")}))
	assert.Equal(t, advisory_model.SeverityLow, advisory.Severity)
}

func TestCollaboratorAndCreditRules(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, db.Insert(ctx, []*repo_model.RepoUnit{{RepoID: 1, Type: unit.TypeSecurityAdvisories}, {RepoID: 3, Type: unit.TypeSecurityAdvisories}}))
	report := &advisory_model.Advisory{RepoID: 1, State: advisory_model.StateTriage, IsReport: true, ReporterID: 4, Summary: "Report", Description: "Details"}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, report))
	require.NoError(t, db.Insert(ctx, &user_model.Blocking{BlockerID: 4, BlockeeID: 5}))
	assert.ErrorIs(t, AddCollaborator(ctx, admin, report, "user5", false), user_model.ErrBlockedUser)

	assert.ErrorIs(t, EditAdvisory(ctx, admin, report, advisory_model.Permissions{CanManage: true},
		EditOptions{Credits: optional.Some([]*advisory_model.Credit{{UserID: 31, Type: "finder"}})}), util.ErrInvalidArgument)

	orgDraft := &advisory_model.Advisory{RepoID: 3, State: advisory_model.StateDraft, ReporterID: admin.ID, Summary: "Draft"}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, orgDraft))
	assert.ErrorIs(t, AddCollaborator(ctx, admin, orgDraft, "test_team", true), util.ErrInvalidArgument)
	require.NoError(t, AddCollaborator(ctx, admin, orgDraft, "team1", true))

	duplicate := &advisory_model.Advisory{RepoID: 1, State: advisory_model.StateTriage, IsReport: true, ReporterID: 8, Summary: "Duplicate"}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, duplicate))
	require.NoError(t, ChangeState(ctx, admin, duplicate, "closed", "duplicate", report.Identifier, false))
	assert.NotZero(t, duplicate.ClosedUnix)
	assert.ErrorIs(t, DeleteAdvisory(ctx, admin, report), util.ErrInvalidArgument)

	other := &advisory_model.Advisory{RepoID: 1, State: advisory_model.StateDraft, ReporterID: admin.ID, Summary: "Another advisory"}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, other))
	reportCopy := *report
	assert.ErrorIs(t, ChangeState(ctx, admin, &reportCopy, "closed", "duplicate", other.Identifier, false), util.ErrInvalidArgument)

	viewer := advisory_model.Viewer{Doer: &user_model.User{ID: duplicate.ReporterID}}
	perms, err := viewer.Permissions(ctx, report)
	require.NoError(t, err)
	assert.False(t, perms.CanView)
	shared := &advisory_model.Advisory{RepoID: 1, State: advisory_model.StateTriage, IsReport: true, ReporterID: 8, Summary: "Shared duplicate"}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, shared))
	require.NoError(t, ChangeState(ctx, admin, shared, "closed", "duplicate", report.Identifier, true))
	perms, err = viewer.Permissions(ctx, report)
	require.NoError(t, err)
	assert.Equal(t, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true}, perms)
	_, err = CreateComment(ctx, viewer.Doer, report, perms, "comment", false)
	assert.ErrorIs(t, err, util.ErrPermissionDenied, "a follower only reads")
	require.NoError(t, ChangeState(ctx, admin, shared, "triage", "", "", false))
	perms, err = viewer.Permissions(ctx, report)
	require.NoError(t, err)
	assert.False(t, perms.CanView)
	require.NoError(t, ChangeState(ctx, admin, shared, "closed", "duplicate", report.Identifier, true))
	require.NoError(t, AddCollaborator(ctx, admin, report, "user8", false))
	require.NoError(t, DeleteAdvisory(ctx, admin, shared))
	require.NoError(t, ChangeState(ctx, admin, duplicate, "draft", "", "", false))
	assert.Zero(t, duplicate.ClosedUnix)
	assert.Zero(t, duplicate.DuplicateOfID)
	perms, err = viewer.Permissions(ctx, report)
	require.NoError(t, err)
	assert.Equal(t, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanComment: true, CanEdit: true}, perms)
	require.NoError(t, DeleteAdvisory(ctx, admin, report))
}
