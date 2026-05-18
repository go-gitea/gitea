// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/test"
	issue_service "code.gitea.io/gitea/services/issue"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setProjectsConfig(t *testing.T, repo *repo_model.Repository, cfg *repo_model.ProjectsConfig) {
	t.Helper()
	repoUnit, err := repo.GetUnit(t.Context(), unit.TypeProjects)
	require.NoError(t, err)
	repoUnit.Config = cfg
	_, err = db.GetEngine(t.Context()).ID(repoUnit.ID).Cols("config").Update(repoUnit)
	require.NoError(t, err)
}

func newRepoProject(t *testing.T, repo *repo_model.Repository, creator *user_model.User, title string) *project_model.Project {
	t.Helper()
	p := &project_model.Project{
		Title:        title,
		RepoID:       repo.ID,
		OwnerID:      repo.OwnerID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeBasicKanban,
		CreatorID:    creator.ID,
	}
	require.NoError(t, project_model.NewProject(t.Context(), p))
	return p
}

// loadIssueProjects reads the issue's projects straight from the DB so each
// assertion sees current state regardless of the issue struct's load state.
func loadIssueProjects(t *testing.T, issue *issues_model.Issue) []*project_model.Project {
	t.Helper()
	var projects []*project_model.Project
	err := db.GetEngine(t.Context()).Table("project").
		Join("INNER", "project_issue", "project.id=project_issue.project_id").
		Where("project_issue.issue_id = ?", issue.ID).
		OrderBy("project.id ASC").
		Find(&projects)
	require.NoError(t, err)
	return projects
}

func assertIssueOnProject(t *testing.T, issue *issues_model.Issue, projectID int64) {
	t.Helper()
	projects := loadIssueProjects(t, issue)
	found := false
	for _, p := range projects {
		if p.ID == projectID {
			found = true
		}
	}
	assert.True(t, found, "issue %d should be on project %d", issue.ID, projectID)
}

func assertIssueOnNoProject(t *testing.T, issue *issues_model.Issue) {
	t.Helper()
	projects := loadIssueProjects(t, issue)
	assert.Empty(t, projects, "issue %d should be on no project", issue.ID)
}

func createTestIssue(t *testing.T, repo *repo_model.Repository, poster *user_model.User, title string, projectIDs []int64) *issues_model.Issue {
	t.Helper()
	issue := &issues_model.Issue{
		RepoID:   repo.ID,
		PosterID: poster.ID,
		Poster:   poster,
		Title:    title,
	}
	require.NoError(t, issue_service.NewIssue(t.Context(), repo, issue, nil, nil, nil, projectIDs))
	return issue
}

// The default project is a PRE-SELECTION on the new-issue page (no server-side
// auto-assignment). A user who loads the page and submits without touching the
// projects widget gets the pre-selected default.
func TestDefaultProjectIssuesPreselectedAndKept(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	proj := newRepoProject(t, repo, owner, "Issues Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: proj.ID,
	})

	session := loginUser(t, owner.Name)
	issue := postNewIssueAcceptingPageDefaults(t, session, repo, "kept preselected default")
	assertIssueOnProject(t, issue, proj.ID)
}

// Sanity: creating an issue NOT through the new-issue page (direct service /
// API path) never gets the default project, because there is no server-side
// auto-assignment — the default lives only in the page pre-selection.
func TestDefaultProjectNotAppliedOutsideNewIssuePage(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	proj := newRepoProject(t, repo, user, "Issues Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: proj.ID,
	})

	issue := createTestIssue(t, repo, user, "service-created issue", nil)
	assertIssueOnNoProject(t, issue)
}

func TestDefaultProjectDontAutoAssign(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	newRepoProject(t, repo, owner, "Unused Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: 0, // 0 = nothing pre-selected
	})

	session := loginUser(t, owner.Name)
	issue := postNewIssueAcceptingPageDefaults(t, session, repo, "no default configured")
	assertIssueOnNoProject(t, issue)
}

// Reproduces issue #28: a default project IS configured (so the new-issue page
// pre-selects it), but the user clears the project before submitting. Because
// the default is only a pre-selection and there is NO server-side
// auto-assignment, the cleared (empty) submission must create an issue with no
// project — the pre-selected default must not come back.
func TestDefaultProjectExplicitClearOnWebFormBeatsDefault(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	proj := newRepoProject(t, repo, owner, "Default Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: proj.ID,
	})

	session := loginUser(t, owner.Name)
	issue := postNewIssueWithProjects(t, session, repo, "cleared on web form", map[string]string{
		"project_ids": "", // field present, explicitly empty == user cleared it
	})
	assertIssueOnNoProject(t, issue)
}

func TestDefaultProjectExplicitSelectionWins(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	defaultProj := newRepoProject(t, repo, user, "Default Board")
	explicitProj := newRepoProject(t, repo, user, "Explicit Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: defaultProj.ID,
	})

	issue := createTestIssue(t, repo, user, "explicit issue", []int64{explicitProj.ID})
	assertIssueOnProject(t, issue, explicitProj.ID)
	// Default must not also be added when an explicit project was chosen.
	for _, p := range loadIssueProjects(t, issue) {
		assert.NotEqual(t, defaultProj.ID, p.ID, "default must not be added when an explicit project was chosen")
	}
}

// A closed project configured as the default must not be pre-selected on the
// new-issue page, so accepting the page defaults yields no project.
func TestDefaultProjectClosedSkipped(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	proj := newRepoProject(t, repo, owner, "Closed Board")
	require.NoError(t, project_model.ChangeProjectStatusByRepoIDAndID(t.Context(), repo.ID, proj.ID, true))
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: proj.ID,
	})

	session := loginUser(t, owner.Name)
	issue := postNewIssueAcceptingPageDefaults(t, session, repo, "closed default not preselected")
	assertIssueOnNoProject(t, issue)
}

// The issue and PR defaults are independent: the new-ISSUE page pre-selects
// the issues default only, never the PR default.
func TestDefaultProjectIssuesAndPRsIndependent(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	issuesProj := newRepoProject(t, repo, owner, "Issues Only Board")
	prsProj := newRepoProject(t, repo, owner, "PRs Only Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:                    repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues:       issuesProj.ID,
		DefaultProjectIDForPullRequests: prsProj.ID,
	})

	session := loginUser(t, owner.Name)
	issue := postNewIssueAcceptingPageDefaults(t, session, repo, "independent issue")
	assertIssueOnProject(t, issue, issuesProj.ID)
	// PR-default must not bleed into a plain issue's pre-selection.
	for _, p := range loadIssueProjects(t, issue) {
		assert.NotEqual(t, prsProj.ID, p.ID, "PR default must not be added to an issue")
	}
}

// Note: the PR-side default ("new-PR page pre-selects the PR default, not the
// issues default; closed/unconfigured -> none") is covered far more cheaply and
// reliably by the service-unit test TestGetDefaultProjectID in
// services/issue/issue_test.go. A compare-page integration test here was tried
// and removed: it required reverse-engineering a valid fixture compare branch
// pair and a real git server, high fragility for logic that is pure and
// unit-testable.

// TestDefaultProjectClearedWhenScopeConflictsWithMode drives the real
// Settings -> Advanced HTTP POST: it submits projects_mode=repo together with
// a default that points at an OWNER-scope project. The client JS filter is
// UX-only; the actual security boundary is handleSettingsPostAdvanced
// re-validating server-side and resetting the conflicting default to 0. This
// proves that boundary end to end.
func TestDefaultProjectClearedWhenScopeConflictsWithMode(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.Name)

	// Owner-scope project: RepoID 0, Type matching an individual owner. It is
	// accessible to the repo's owner, so the only reason it must be cleared
	// is the mode/scope conflict (owner project under repo-only mode).
	ownerProj := &project_model.Project{
		Title:        "Owner Scope Board",
		RepoID:       0,
		OwnerID:      repo.OwnerID,
		Type:         project_model.TypeIndividual,
		TemplateType: project_model.TemplateTypeBasicKanban,
		CreatorID:    owner.ID,
	}
	require.NoError(t, project_model.NewProject(t.Context(), ownerProj))

	// enable_pulls is intentionally omitted: with pulls disabled the handler
	// skips PullRequestsConfig.ValidateUpdateSettings entirely (upstream
	// #37410) so the POST reaches the projects clear path unmasked. The
	// projects unit alone keeps len(units) > 0 so the handler persists.
	req := NewRequestWithValues(t, "POST", repo.Link()+"/settings", map[string]string{
		"action":                        "advanced",
		"repo_name":                     repo.Name,
		"enable_code":                   "on",
		"enable_projects":               "on",
		"projects_mode":                 "repo",
		"default_project_id_for_issues": strconv.FormatInt(ownerProj.ID, 10),
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	projectsUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{
		RepoID: repo.ID,
		Type:   unit.TypeProjects,
	})
	cfg := projectsUnit.ProjectsConfig()
	// The projects unit was actually written (mode persisted)...
	assert.Equal(t, repo_model.ProjectsModeRepo, cfg.GetProjectsMode())
	// ...and the owner-scope default was authoritatively cleared by the server.
	assert.EqualValues(t, 0, cfg.GetDefaultProjectIDForIssues(),
		"owner-scope default must be cleared under repo mode")
}

// postNewIssueWithProjects mirrors the proven testNewIssue request pattern
// (GET the new-issue page, submit the rendered form's action so CSRF and
// route are exactly what the UI uses), but adds project/column form fields.
func postNewIssueWithProjects(t *testing.T, session *TestSession, repo *repo_model.Repository, title string, extra map[string]string) *issues_model.Issue {
	t.Helper()
	req := NewRequest(t, "GET", repo.Link()+"/issues/new")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	require.True(t, exists, "The new-issue template has changed")

	values := map[string]string{
		"title":   title,
		"content": "body",
	}
	maps.Copy(values, extra)
	req = NewRequestWithValues(t, "POST", link, values)
	resp = session.MakeRequest(t, req, http.StatusOK)

	issueURL := test.RedirectURL(resp)
	require.NotEmpty(t, issueURL, "create-issue did not redirect (validation likely failed)")

	return unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Title: title})
}

// postNewIssueAcceptingPageDefaults loads the new-issue page and submits it
// with the project_ids value EXACTLY as the server pre-rendered it (the user
// does not touch the projects widget). This is the faithful test of the
// "default project is a pre-selection" model: there is no server-side
// auto-assignment, so an issue only lands on the default project if the page
// pre-selected it and the user submitted that pre-selection unchanged.
func postNewIssueAcceptingPageDefaults(t *testing.T, session *TestSession, repo *repo_model.Repository, title string) *issues_model.Issue {
	t.Helper()
	req := NewRequest(t, "GET", repo.Link()+"/issues/new")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	require.True(t, exists, "The new-issue template has changed")
	// replay the server's pre-selected project_ids verbatim (may be empty)
	prefilledProjectIDs, _ := htmlDoc.doc.Find(`input[name="project_ids"]`).Attr("value")

	values := map[string]string{
		"title":       title,
		"content":     "body",
		"project_ids": prefilledProjectIDs,
	}
	req = NewRequestWithValues(t, "POST", link, values)
	resp = session.MakeRequest(t, req, http.StatusOK)

	issueURL := test.RedirectURL(resp)
	require.NotEmpty(t, issueURL, "create-issue did not redirect (validation likely failed)")

	return unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Title: title})
}

// The repo-default pre-selection must NOT hijack the post-create redirect: a
// plainly-created issue (default merely pre-selected) lands on the issue, while
// arriving in an explicit single-project context via ?project= still returns to
// the project board (preserved upstream behavior).
func TestDefaultProjectPreselectionDoesNotHijackRedirect(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	proj := newRepoProject(t, repo, owner, "Redirect Board")
	setProjectsConfig(t, repo, &repo_model.ProjectsConfig{
		ProjectsMode:              repo_model.ProjectsModeRepo,
		DefaultProjectIDForIssues: proj.ID,
	})
	session := loginUser(t, owner.Name)

	submit := func(t *testing.T, getURL, title string) string {
		t.Helper()
		resp := session.MakeRequest(t, NewRequest(t, "GET", getURL), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		link, ok := htmlDoc.doc.Find("form.ui.form").Attr("action")
		require.True(t, ok)
		projectIDs, _ := htmlDoc.doc.Find(`input[name="project_ids"]`).Attr("value")
		redirectAfter, _ := htmlDoc.doc.Find(`input[name="redirect_after_creation"]`).Attr("value")
		resp = session.MakeRequest(t, NewRequestWithValues(t, "POST", link, map[string]string{
			"title":                   title,
			"content":                 "body",
			"project_ids":             projectIDs,
			"redirect_after_creation": redirectAfter,
		}), http.StatusOK)
		return test.RedirectURL(resp)
	}

	t.Run("plain create with preselected default redirects to the issue", func(t *testing.T) {
		dest := submit(t, repo.Link()+"/issues/new", "plain redirect issue")
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Title: "plain redirect issue"})
		assertIssueOnProject(t, issue, proj.ID) // still pre-selected & assigned
		// build the issue URL from repo (loaded) + index; issue.Link() would
		// deref issue.Repo which AssertExistsAndLoadBean does not populate.
		wantIssueURL := fmt.Sprintf("%s/issues/%d", repo.Link(), issue.Index)
		assert.Equal(t, wantIssueURL, dest, "should land on the new issue, not the project board")
	})

	t.Run("explicit ?project= context still redirects to the board", func(t *testing.T) {
		dest := submit(t, repo.Link()+"/issues/new?project="+strconv.FormatInt(proj.ID, 10), "board redirect issue")
		assert.Equal(t, project_model.ProjectLinkForRepo(repo, proj.ID), dest,
			"explicit single-project context must still return to the project board")
	})
}

func TestCreateIssueWithChosenProjectColumn(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	proj := newRepoProject(t, repo, owner, "Board")
	cols, err := proj.GetColumns(t.Context())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(cols), 2)
	defCol, err := proj.MustDefaultColumn(t.Context())
	require.NoError(t, err)
	var target *project_model.Column
	for _, c := range cols {
		if c.ID != defCol.ID {
			target = c
			break
		}
	}
	require.NotNil(t, target)

	session := loginUser(t, owner.Name)
	issue := postNewIssueWithProjects(t, session, repo, "issue with chosen column", map[string]string{
		"project_ids":       strconv.FormatInt(proj.ID, 10),
		"project_board_ids": strconv.FormatInt(proj.ID, 10) + ":" + strconv.FormatInt(target.ID, 10),
	})

	pi := unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: issue.ID, ProjectID: proj.ID})
	assert.Equal(t, target.ID, pi.ProjectColumnID)
}

// Note: the "no project_board_ids -> default column" case is covered far more
// cheaply by the models/issues unit test
// TestIssueAssignOrRemoveProjectColumn/DefaultColumnWhenNoMap. The two
// integration tests kept here are an intentional minimal end-to-end smoke
// (wiring + security boundary); behavior matrices live in JS/Go unit tests.
func TestCreateIssueForgedProjectBoardIDsFallsBackToDefault(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	proj := newRepoProject(t, repo, owner, "Board3")
	defCol, err := proj.MustDefaultColumn(t.Context())
	require.NoError(t, err)

	session := loginUser(t, owner.Name)
	issue := postNewIssueWithProjects(t, session, repo, "issue forged column", map[string]string{
		"project_ids":       strconv.FormatInt(proj.ID, 10),
		"project_board_ids": strconv.FormatInt(proj.ID, 10) + ":99999",
	})

	pi := unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: issue.ID, ProjectID: proj.ID})
	assert.Equal(t, defCol.ID, pi.ProjectColumnID)
}
