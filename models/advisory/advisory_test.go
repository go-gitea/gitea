// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory_test

import (
	"slices"
	"testing"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateTransitions(t *testing.T) {
	assert.True(t, advisory_model.StateTriage.CanTransitionTo(advisory_model.StateDraft))
	assert.False(t, advisory_model.StateTriage.CanTransitionTo(advisory_model.StatePublished), "reports are accepted as drafts first")
	assert.False(t, advisory_model.StatePublished.CanTransitionTo(advisory_model.StateDraft), "published advisories can only be withdrawn")
	for s := advisory_model.StateTriage; s <= advisory_model.StateWithdrawn; s++ {
		assert.Equal(t, s, advisory_model.ParseState(s.String()))
	}
}

func TestSeverityLocaleKey(t *testing.T) {
	assert.Equal(t, "repo.security_advisories.severity.moderate", advisory_model.SeverityMedium.LocaleKey())
	assert.Equal(t, "repo.security_advisories.severity.unknown", advisory_model.SeverityUnknown.LocaleKey())
	assert.Equal(t, "repo.security_advisories.severity.high", advisory_model.SeverityHigh.LocaleKey())
}

func TestCreateAndDeleteAdvisory(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	a := &advisory_model.Advisory{
		RepoID:          1,
		Summary:         "XSS in markdown",
		State:           advisory_model.StateDraft,
		ReporterID:      2,
		CweIDs:          []string{"CWE-79"},
		Vulnerabilities: []*advisory_model.Vulnerability{{Ecosystem: "go", PackageName: "example.com/pkg", VulnerableVersionRange: "< 1.2.3"}},
		Credits:         []*advisory_model.Credit{{UserID: 4, Type: "finder"}},
	}
	require.NoError(t, advisory_model.CreateAdvisory(t.Context(), a))
	assert.Regexp(t, `^[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}$`, a.Identifier)

	got, err := advisory_model.GetAdvisoryByIdentifier(t.Context(), 1, a.Identifier)
	require.NoError(t, err)
	require.NoError(t, got.LoadAttributes(t.Context()))
	assert.Equal(t, []string{"CWE-79"}, got.CweIDs)
	require.Len(t, got.Vulnerabilities, 1)
	assert.Equal(t, "example.com/pkg", got.Vulnerabilities[0].PackageName)
	require.Len(t, got.Credits, 1)
	assert.Equal(t, "user4", got.Credits[0].User.Name)
	assert.Equal(t, "/user2/repo1/security/advisories/"+a.Identifier, got.Link())

	_, err = advisory_model.GetAdvisoryByIdentifier(t.Context(), 2, a.Identifier)
	assert.ErrorIs(t, err, util.ErrNotExist)

	// a save based on an outdated version is rejected
	stale := *got
	require.NoError(t, advisory_model.UpdateAdvisory(t.Context(), got, "summary"))
	assert.ErrorIs(t, advisory_model.UpdateAdvisory(t.Context(), &stale, "summary"), advisory_model.ErrAdvisoryChanged)

	// a concurrent request must not apply the same transition again
	beforePublish := *got
	a.State = advisory_model.StatePublished
	require.NoError(t, advisory_model.UpdateAdvisoryState(t.Context(), a, advisory_model.StateDraft))
	assert.ErrorIs(t, advisory_model.UpdateAdvisoryState(t.Context(), a, advisory_model.StateDraft), advisory_model.ErrAdvisoryChanged)
	assert.ErrorIs(t, advisory_model.UpdateAdvisory(t.Context(), &beforePublish, "summary"), advisory_model.ErrAdvisoryChanged, "an edit permitted before publishing must fail")

	for _, c := range []*advisory_model.Collaborator{{UserID: 4}, {UserID: 5}, {TeamID: 7}} {
		c.AdvisoryID = a.ID
		_, err := advisory_model.AddCollaborator(t.Context(), c)
		require.NoError(t, err)
	}
	require.NoError(t, advisory_model.DeleteTeamCollaboratorsByRepoID(t.Context(), 1))
	require.NoError(t, advisory_model.DeleteUserCollaboratorsByOwnerID(t.Context(), 4, 2))
	unittest.AssertCount(t, &advisory_model.Collaborator{AdvisoryID: a.ID}, 1)
	unittest.AssertExistsAndLoadBean(t, &advisory_model.Collaborator{AdvisoryID: a.ID, UserID: 5})

	assert.ErrorIs(t, advisory_model.DeleteAdvisory(t.Context(), got), util.ErrInvalidArgument, "it has been published in the meantime")

	require.NoError(t, advisory_model.CreateComment(t.Context(), &advisory_model.Comment{AdvisoryID: a.ID, PosterID: 2, Content: "hi"}))
	require.NoError(t, advisory_model.DeleteAdvisoriesByRepoID(t.Context(), 1))
	unittest.AssertNotExistsBean(t, &advisory_model.Advisory{ID: a.ID})
	unittest.AssertNotExistsBean(t, &advisory_model.Vulnerability{AdvisoryID: a.ID})
	unittest.AssertNotExistsBean(t, &advisory_model.Credit{AdvisoryID: a.ID})
	unittest.AssertNotExistsBean(t, &advisory_model.Comment{AdvisoryID: a.ID})
}

func TestViewerAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := func(id int64) *user_model.User { return unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id}) }

	newAdvisory := func(state advisory_model.State) *advisory_model.Advisory {
		a := &advisory_model.Advisory{RepoID: repo.ID, Summary: state.String(), State: state, ReporterID: 4, IsReport: true}
		require.NoError(t, advisory_model.CreateAdvisory(ctx, a))
		return a
	}
	triage := newAdvisory(advisory_model.StateTriage)
	published := newAdvisory(advisory_model.StatePublished)
	_, err := advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: triage.ID, UserID: 5})
	require.NoError(t, err)
	_, err = advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: triage.ID, TeamID: 7}) // team 7 has member user15
	require.NoError(t, err)
	_, err = advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: triage.ID, UserID: 10, ReadOnly: true})
	require.NoError(t, err)
	// a draft created by user 9 while being an admin, who is no admin anymore
	draft := &advisory_model.Advisory{RepoID: repo.ID, Summary: "draft", State: advisory_model.StateDraft, ReporterID: 9}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, draft))

	cases := []struct {
		name          string
		viewer        advisory_model.Viewer
		perms         advisory_model.Permissions
		canManage     bool
		visibleTriage bool
	}{
		{"anonymous", advisory_model.Viewer{}, advisory_model.Permissions{}, false, false},
		{"repo admin", advisory_model.Viewer{Doer: user(2), IsRepoAdmin: true}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true, CanManage: true}, true, true},
		{"repo admin with public-only token", advisory_model.Viewer{Doer: user(2), IsRepoAdmin: true, PublicOnly: true}, advisory_model.Permissions{}, false, false},
		{"reporter", advisory_model.Viewer{Doer: user(4)}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true}, false, true},
		{"user collaborator", advisory_model.Viewer{Doer: user(5)}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true}, false, true},
		{"team collaborator", advisory_model.Viewer{Doer: user(15)}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true}, false, true},
		{"read-only collaborator", advisory_model.Viewer{Doer: user(10)}, advisory_model.Permissions{CanView: true, CanSeeDiscussion: true}, false, true},
		{"outsider", advisory_model.Viewer{Doer: user(8)}, advisory_model.Permissions{}, false, false},
		{"actions user", advisory_model.Viewer{Doer: user_model.NewActionsUser(), IsRepoAdmin: true}, advisory_model.Permissions{}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			perms, err := c.viewer.Permissions(ctx, triage)
			require.NoError(t, err)
			assert.Equal(t, c.perms, perms)
			assert.Equal(t, c.canManage, c.viewer.CanManage())

			perms, err = c.viewer.Permissions(ctx, published)
			require.NoError(t, err)
			assert.True(t, perms.CanView)
			assert.Equal(t, c.canManage, perms.CanEdit, "only admins edit published advisories")

			list, err := db.Find[advisory_model.Advisory](ctx, advisory_model.FindAdvisoriesOptions{RepoID: repo.ID, Viewer: c.viewer})
			require.NoError(t, err)
			ids := make([]int64, 0, len(list))
			for _, a := range list {
				ids = append(ids, a.ID)
			}
			assert.Contains(t, ids, published.ID)
			assert.Equal(t, c.visibleTriage, slices.Contains(ids, triage.ID))

			counts, err := advisory_model.CountAdvisoriesByState(ctx, advisory_model.FindAdvisoriesOptions{RepoID: repo.ID, Viewer: c.viewer})
			require.NoError(t, err)
			assert.Equal(t, c.visibleTriage, counts[advisory_model.StateTriage] == 1)
		})
	}

	perms, err := advisory_model.Viewer{Doer: user(9)}.Permissions(ctx, draft)
	require.NoError(t, err)
	assert.False(t, perms.CanView, "creating a draft as admin grants no access of its own")

	// leaving the team revokes the access
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

	xss := &advisory_model.Advisory{
		RepoID: 1, Summary: "XSS in markdown", State: advisory_model.StatePublished, ReporterID: 2, Severity: advisory_model.SeverityHigh,
		CweIDs: []string{"CWE-79"}, Labels: []*issues_model.Label{label},
		Vulnerabilities: []*advisory_model.Vulnerability{{Ecosystem: "npm", PackageName: "markdown-it"}},
	}
	dup := &advisory_model.Advisory{RepoID: 1, Summary: "Same XSS", State: advisory_model.StateClosed, CloseReason: advisory_model.CloseReasonDuplicate, ReporterID: 2}
	for _, a := range []*advisory_model.Advisory{xss, dup} {
		require.NoError(t, advisory_model.CreateAdvisory(ctx, a))
	}

	find := func(opts advisory_model.FindAdvisoriesOptions) []int64 {
		opts.RepoID, opts.Viewer = 1, advisory_model.Viewer{Doer: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}), IsRepoAdmin: true}
		list, err := db.Find[advisory_model.Advisory](ctx, opts)
		require.NoError(t, err)
		ids := make([]int64, 0, len(list))
		for _, a := range list {
			ids = append(ids, a.ID)
		}
		return ids
	}
	assert.ElementsMatch(t, []int64{xss.ID, dup.ID}, find(advisory_model.FindAdvisoriesOptions{Keyword: "xss"}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Keyword: "MARKDOWN-IT"}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Keyword: xss.Identifier}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Severity: advisory_model.SeverityHigh}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{label.ID}}))
	assert.Equal(t, []int64{dup.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{-label.ID}}))
	assert.Equal(t, []int64{dup.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{0}}))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{Ecosystem: "npm", CweID: "cwe-79"}))
	assert.Empty(t, find(advisory_model.FindAdvisoriesOptions{CweID: "CWE-7"}))
	assert.Equal(t, []int64{dup.ID}, find(advisory_model.FindAdvisoriesOptions{CloseReason: advisory_model.CloseReasonDuplicate}))
	require.NoError(t, advisory_model.DeleteLabelLinks(ctx, label.ID))
	assert.Equal(t, []int64{xss.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{label.ID}}), "existing labels stay assigned")
	require.NoError(t, issues_model.DeleteLabel(ctx, label.RepoID, label.ID))
	require.NoError(t, advisory_model.DeleteLabelLinks(ctx, label.ID))
	assert.ElementsMatch(t, []int64{xss.ID, dup.ID}, find(advisory_model.FindAdvisoriesOptions{LabelIDs: []int64{0}}), "deleted labels are unassigned")

	counts, err := advisory_model.CountAdvisoriesByState(ctx, advisory_model.FindAdvisoriesOptions{
		RepoID: 1, Viewer: advisory_model.Viewer{Doer: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}), IsRepoAdmin: true},
		Keyword: "same", States: []advisory_model.State{advisory_model.StatePublished},
	})
	require.NoError(t, err)
	assert.Equal(t, map[advisory_model.State]int64{advisory_model.StateClosed: 1}, counts, "the state is ignored, the other filters are applied")
}
