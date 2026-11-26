// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory_test

import (
	"slices"
	"strings"
	"testing"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createAdvisory(t *testing.T, advisory *advisory_model.Advisory) *advisory_model.Advisory {
	t.Helper()
	require.NoError(t, advisory_model.CreateAdvisory(t.Context(), advisory))
	return advisory
}

func findAdvisoryIDs(t *testing.T, opts advisory_model.FindAdvisoriesOptions) []int64 {
	t.Helper()
	list, err := db.Find[advisory_model.Advisory](t.Context(), opts)
	require.NoError(t, err)
	ids := make([]int64, 0, len(list))
	for _, advisory := range list {
		ids = append(ids, advisory.ID)
	}
	return ids
}

func TestStateTransitions(t *testing.T) {
	assert.True(t, advisory_model.StateTriage.CanTransitionTo(advisory_model.StateDraft))
	assert.False(t, advisory_model.StateTriage.CanTransitionTo(advisory_model.StatePublished), "reports are accepted as drafts first")
	assert.False(t, advisory_model.StatePublished.CanTransitionTo(advisory_model.StateDraft), "published advisories can only be withdrawn")
	for state := advisory_model.StateTriage; state <= advisory_model.StateWithdrawn; state++ {
		assert.Equal(t, state, advisory_model.ParseState(state.String()))
	}
}

func TestSeverityLocaleKey(t *testing.T) {
	assert.Equal(t, "repo.security_advisories.severity.moderate", advisory_model.SeverityMedium.LocaleKey())
	assert.Equal(t, "repo.security_advisories.severity.unknown", advisory_model.SeverityUnknown.LocaleKey())
	assert.Equal(t, "repo.security_advisories.severity.high", advisory_model.SeverityHigh.LocaleKey())
}

func TestAdvisoryPersistence(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	advisory := createAdvisory(t, &advisory_model.Advisory{
		RepoID:          1,
		Summary:         "XSS in markdown",
		State:           advisory_model.StateDraft,
		ReporterID:      2,
		CvssVector:      "CVSS:4.0/AV:L/AC:L/AT:N/PR:L/UI:P/VC:N/VI:H/VA:H/SC:N/SI:L/SA:L",
		CvssScoreTenths: 52,
		CweIDs:          []string{"CWE-79"},
		Vulnerabilities: []*advisory_model.Vulnerability{{Ecosystem: "go", PackageName: "example.com/pkg", VulnerableVersionRange: "< 1.2.3"}},
		Credits:         []*advisory_model.Credit{{UserID: 4, Type: "finder"}},
	})
	assert.Regexp(t, `^GTSA(-[23456789cfghjmpqrvwx]{4}){3}$`, advisory.Identifier)
	assert.EqualValues(t, 1, advisory.Index)
	assert.EqualValues(t, 2, createAdvisory(t, &advisory_model.Advisory{RepoID: 1, Summary: "next", State: advisory_model.StateDraft, ReporterID: 2}).Index)
	assert.EqualValues(t, 1, createAdvisory(t, &advisory_model.Advisory{RepoID: 2, Summary: "other repository", State: advisory_model.StateDraft, ReporterID: 2}).Index)

	got, err := advisory_model.GetAdvisoryByIdentifier(t.Context(), 1, strings.ToUpper(advisory.Identifier))
	require.NoError(t, err)
	require.NoError(t, got.LoadAttributes(t.Context()))
	assert.Equal(t, advisory.CvssVector, got.CvssVector)
	assert.Equal(t, cvss.Version40, got.CvssVersion())
	assert.InDelta(t, 5.2, got.CvssScore(), 1e-9)
	assert.Equal(t, []string{"CWE-79"}, got.CweIDs)
	require.Len(t, got.Vulnerabilities, 1)
	assert.Equal(t, "example.com/pkg", got.Vulnerabilities[0].PackageName)
	require.Len(t, got.Credits, 1)
	assert.Equal(t, "user4", got.Credits[0].User.Name)
	assert.Equal(t, "/user2/repo1/security/advisories/"+advisory.Identifier, got.Link())

	_, err = advisory_model.GetAdvisoryByIdentifier(t.Context(), 2, advisory.Identifier)
	assert.ErrorIs(t, err, util.ErrNotExist)

	stale := *got
	require.NoError(t, advisory_model.UpdateAdvisory(t.Context(), got))
	assert.ErrorIs(t, advisory_model.UpdateAdvisory(t.Context(), &stale), advisory_model.ErrAdvisoryChanged)

	beforePublish := *got
	advisory.State = advisory_model.StatePublished
	require.NoError(t, advisory_model.UpdateAdvisoryState(t.Context(), advisory, advisory_model.StateDraft))
	assert.ErrorIs(t, advisory_model.UpdateAdvisoryState(t.Context(), advisory, advisory_model.StateDraft), advisory_model.ErrAdvisoryChanged)
	assert.ErrorIs(t, advisory_model.UpdateAdvisory(t.Context(), &beforePublish), advisory_model.ErrAdvisoryChanged, "an edit permitted before publishing must fail")

	for _, collaborator := range []*advisory_model.Collaborator{{UserID: 4}, {UserID: 5}, {TeamID: 7}} {
		collaborator.AdvisoryID = advisory.ID
		_, err := advisory_model.AddCollaborator(t.Context(), collaborator)
		require.NoError(t, err)
	}
	require.NoError(t, advisory_model.DeleteTeamCollaboratorsByRepoID(t.Context(), 1))
	require.NoError(t, advisory_model.DeleteUserCollaboratorsByOwnerID(t.Context(), 4, 2))
	unittest.AssertCount(t, &advisory_model.Collaborator{AdvisoryID: advisory.ID}, 1)
	unittest.AssertExistsAndLoadBean(t, &advisory_model.Collaborator{AdvisoryID: advisory.ID, UserID: 5})

	assert.ErrorIs(t, advisory_model.DeleteAdvisory(t.Context(), got), util.ErrInvalidArgument, "it has been published in the meantime")

	require.NoError(t, db.Insert(t.Context(), &advisory_model.Comment{AdvisoryID: advisory.ID, PosterID: 2, Content: "hi"}))
	require.NoError(t, advisory_model.DeleteAdvisoriesByRepoID(t.Context(), 1))
	unittest.AssertNotExistsBean(t, &advisory_model.Advisory{ID: advisory.ID})
	unittest.AssertNotExistsBean(t, &advisory_model.Vulnerability{AdvisoryID: advisory.ID})
	unittest.AssertNotExistsBean(t, &advisory_model.Credit{AdvisoryID: advisory.ID})
	unittest.AssertNotExistsBean(t, &advisory_model.Comment{AdvisoryID: advisory.ID})
}

func TestViewerAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := func(id int64) *user_model.User { return &user_model.User{ID: id} }

	triage := createAdvisory(t, &advisory_model.Advisory{RepoID: repo.ID, Summary: "triage", State: advisory_model.StateTriage, ReporterID: 4, IsReport: true})
	published := createAdvisory(t, &advisory_model.Advisory{RepoID: repo.ID, Summary: "published", State: advisory_model.StatePublished, ReporterID: 4, IsReport: true})
	for _, collaborator := range []*advisory_model.Collaborator{{UserID: 5}, {TeamID: 7}} {
		collaborator.AdvisoryID = triage.ID
		_, err := advisory_model.AddCollaborator(ctx, collaborator)
		require.NoError(t, err)
	}
	draft := createAdvisory(t, &advisory_model.Advisory{RepoID: repo.ID, Summary: "draft", State: advisory_model.StateDraft, ReporterID: 9})

	collaboratorPerms := advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanSeeConfidential: true, CanEdit: true}
	for _, viewerCase := range []struct {
		name   string
		viewer advisory_model.Viewer
		perms  advisory_model.Permissions
	}{
		{"anonymous", advisory_model.Viewer{}, advisory_model.Permissions{}},
		{"repo admin", advisory_model.Viewer{Doer: user(2), IsRepoAdmin: true}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanSeeConfidential: true, CanEdit: true, CanManage: true}},
		{"security team member", advisory_model.Viewer{Doer: user(8), IsSecurityTeamMember: true}, collaboratorPerms},
		{"repo admin with public-only token", advisory_model.Viewer{Doer: user(2), IsRepoAdmin: true, PublicOnly: true}, advisory_model.Permissions{}},
		{"reporter", advisory_model.Viewer{Doer: user(4)}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true}},
		{"user collaborator", advisory_model.Viewer{Doer: user(5)}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true}},
		{"user collaborator with write access", advisory_model.Viewer{Doer: user(5), IsRepoWriter: true}, collaboratorPerms},
		{"team collaborator with write access", advisory_model.Viewer{Doer: user(15), IsRepoWriter: true}, collaboratorPerms},
		{"outsider", advisory_model.Viewer{Doer: user(8)}, advisory_model.Permissions{}},
		{"actions user", advisory_model.Viewer{Doer: user_model.NewActionsUser(), IsRepoAdmin: true}, advisory_model.Permissions{}},
	} {
		t.Run(viewerCase.name, func(t *testing.T) {
			perms, err := viewerCase.viewer.Permissions(ctx, triage)
			require.NoError(t, err)
			assert.Equal(t, viewerCase.perms, perms)
			assert.Equal(t, viewerCase.perms.CanManage, viewerCase.viewer.CanManage())

			perms, err = viewerCase.viewer.Permissions(ctx, published)
			require.NoError(t, err)
			assert.True(t, perms.CanView)
			assert.Equal(t, viewerCase.perms.CanManage, perms.CanEdit, "only admins edit published advisories")

			opts := advisory_model.FindAdvisoriesOptions{RepoID: repo.ID, Viewer: viewerCase.viewer}
			ids := findAdvisoryIDs(t, opts)
			assert.Contains(t, ids, published.ID)
			assert.Equal(t, viewerCase.perms.CanView, slices.Contains(ids, triage.ID))
		})
	}

	perms, err := advisory_model.Viewer{Doer: user(9)}.Permissions(ctx, draft)
	require.NoError(t, err)
	assert.False(t, perms.CanView, "creating a draft as admin grants no access of its own")

	duplicate := createAdvisory(t, &advisory_model.Advisory{RepoID: repo.ID, Summary: "duplicate", State: advisory_model.StateClosed, ReporterID: 4, IsReport: true, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOfID: draft.ID})
	_, err = advisory_model.Viewer{Doer: user(4)}.LoadForDisplay(ctx, advisory_model.List{duplicate})
	require.NoError(t, err)
	require.NotNil(t, duplicate.DuplicateOf)
	assert.True(t, duplicate.DuplicateOf.IsMetadataOnly)
	assert.Equal(t, draft.Identifier, duplicate.DuplicateOf.Identifier)
	assert.Equal(t, advisory_model.StateDraft, duplicate.DuplicateOf.State)
	assert.Empty(t, duplicate.DuplicateOf.Summary)

	_, err = advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: triage.ID, UserID: 4})
	require.NoError(t, err)
	perms, err = advisory_model.Viewer{Doer: user(4)}.Permissions(ctx, triage)
	require.NoError(t, err)
	assert.False(t, perms.CanSeeConfidential, "a reporter without write access doesn't see confidential comments, even as a collaborator")

	_, err = db.GetEngine(ctx).Exec("DELETE FROM team_user WHERE team_id = 7 AND uid = 15")
	require.NoError(t, err)
	perms, err = advisory_model.Viewer{Doer: user(15)}.Permissions(ctx, triage)
	require.NoError(t, err)
	assert.False(t, perms.CanView)
}

func TestFindAdvisoriesFilters(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1, RepoID: 1})
	admin := advisory_model.Viewer{Doer: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}), IsRepoAdmin: true}

	xss := createAdvisory(t, &advisory_model.Advisory{
		RepoID: 1, Summary: "XSS in markdown", State: advisory_model.StatePublished, ReporterID: 2, Severity: advisory_model.SeverityHigh,
		CweIDs: []string{"CWE-79"}, Labels: []*issues_model.Label{label},
		Vulnerabilities: []*advisory_model.Vulnerability{{Ecosystem: "npm", PackageName: "markdown-it"}},
	})
	dup := createAdvisory(t, &advisory_model.Advisory{RepoID: 1, Summary: "Same XSS", State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, ReporterID: 2})

	find := func(opts advisory_model.FindAdvisoriesOptions) []int64 {
		opts.RepoID, opts.Viewer = 1, admin
		return findAdvisoryIDs(t, opts)
	}
	assert.ElementsMatch(t, []int64{xss.ID, dup.ID}, find(advisory_model.FindAdvisoriesOptions{Keyword: "xss"}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Keyword: "MARKDOWN-IT"}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Keyword: xss.Identifier}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Severity: advisory_model.SeverityHigh}))
	assert.Equal(t, []int64{dup.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{-label.ID}}))
	assert.Equal(t, []int64{dup.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{0}}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Ecosystem: "npm", CweID: "cwe-79"}))
	assert.Empty(t, find(advisory_model.FindAdvisoriesOptions{CweID: "CWE-7"}))
	assert.Equal(t, []int64{dup.ID}, find(advisory_model.FindAdvisoriesOptions{CloseReason: advisory_model.CloseReasonDuplicate}))
	require.NoError(t, advisory_model.DeleteLabelLinks(ctx, label.ID))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{label.ID}}), "existing labels stay assigned")
	require.NoError(t, issues_model.DeleteRepoLabel(ctx, label.RepoID, label.ID))
	require.NoError(t, advisory_model.DeleteLabelLinks(ctx, label.ID))
	assert.ElementsMatch(t, []int64{xss.ID, dup.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{0}}), "deleted labels are unassigned")

	counts, err := advisory_model.CountAdvisoriesByState(ctx, advisory_model.FindAdvisoriesOptions{RepoID: 1, Viewer: admin, Keyword: "same", States: []advisory_model.State{advisory_model.StatePublished}})
	require.NoError(t, err)
	assert.Equal(t, map[advisory_model.State]int64{advisory_model.StateClosed: 1}, counts, "the state is ignored, the other filters are applied")

	createAdvisory(t, &advisory_model.Advisory{RepoID: 1, Summary: "Same XSS again", State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, DuplicateOfID: xss.ID, ReporterID: 2})
	candidates, err := advisory_model.FindDuplicateCandidates(ctx, dup, admin)
	require.NoError(t, err)
	require.Len(t, candidates, 1, "neither the advisory itself nor a duplicate can be the original")
	assert.Equal(t, xss.Identifier, candidates[0].Identifier)
	assert.Zero(t, candidates[0].ID, "only what the form shows is loaded")
}
