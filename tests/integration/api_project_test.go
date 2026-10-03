// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectScope runs the same lifecycle against each owner type, so scope-specific routing
// or permission mistakes surface without triplicating the test body.
type projectScope struct {
	name string
	base string
	// an issue whose repository the scope's owner may place on its board
	issueID int64
	// a second such issue, deliberately left off the board
	foreignIssueID int64
}

func TestAPIProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 owns repo1 and is on org3's Owners team, so one token covers all three scopes
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization,
		auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
	// user8 is signed in but is neither an org member nor a repo1 collaborator, and is scoped
	// generously so that permissions rather than token scopes are what denies below
	outsider := getUserToken(t, "user8", auth_model.AccessTokenScopeWriteIssue,
		auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeReadUser)

	for _, scope := range []projectScope{
		{"Repository", "/api/v1/repos/user2/repo1/projects", 1, 11},
		{"Organization", "/api/v1/orgs/org3/projects", 16, 17},
		{"User", "/api/v1/user/projects", 1, 11},
	} {
		t.Run(scope.name, func(t *testing.T) {
			testProjectLifecycle(t, scope, token)
		})
	}

	t.Run("ListOtherUserProjects", func(t *testing.T) {
		// no fixture project is individual, so create one: without it the route answers with an
		// empty list and the assertions below would pass vacuously
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/projects", &api.CreateProjectOption{Title: "individual"}).AddTokenAuth(token)
		created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Project{})

		req = NewRequest(t, "GET", "/api/v1/users/user2/projects").AddTokenAuth(token)
		projects := *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]*api.Project{})
		require.Len(t, projects, 1)
		assert.Equal(t, created.ID, projects[0].ID)
		assert.Equal(t, "individual", projects[0].Type)
	})

	t.Run("DefaultColumnHoldsUnassignedIssues", func(t *testing.T) {
		// fixture issue 2 carries project_board_id=0, which the board shows in the default column
		defaultColumn := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ProjectID: 1, Default: true})
		req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/projects/1/columns/%d/issues", defaultColumn.ID).AddTokenAuth(token)
		issueIDs := make([]int64, 0)
		for _, issue := range *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]api.Issue{}) {
			issueIDs = append(issueIDs, issue.ID)
		}
		assert.Contains(t, issueIDs, int64(2))

		// and the same column must accept removing what it lists
		req = NewRequestf(t, "DELETE", "/api/v1/repos/user2/repo1/projects/1/columns/%d/issues/2", defaultColumn.ID).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		unittest.AssertNotExistsBean(t, &project_model.ProjectIssue{ProjectID: 1, IssueID: 2})
	})

	t.Run("Permissions", func(t *testing.T) { testAPIProjectPermissions(t, token, outsider) })
	t.Run("Visibility", func(t *testing.T) { testAPIProjectVisibility(t, outsider) })
	t.Run("RepoProjectsModeOwner", func(t *testing.T) { testAPIRepoProjectsModeOwner(t, token) })
}

func testProjectLifecycle(t *testing.T, scope projectScope, token string) {
	req := NewRequestWithJSON(t, "POST", scope.base, &api.CreateProjectOption{
		Title:        "lifecycle",
		Description:  "created via API",
		TemplateType: "basic_kanban",
		CardType:     "images_and_text",
	}).AddTokenAuth(token)
	project := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Project{})
	assert.Equal(t, "lifecycle", project.Title)
	assert.Equal(t, "basic_kanban", project.TemplateType)
	assert.Equal(t, "images_and_text", project.CardType)
	assert.Equal(t, api.StateOpen, project.State)
	assert.NotEmpty(t, project.HTMLURL)
	projectURL := fmt.Sprintf("%s/%d", scope.base, project.ID)

	req = NewRequest(t, "GET", scope.base+"?state=open").AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.NotEmpty(t, resp.Header().Get("X-Total-Count"))
	listedIDs := make([]int64, 0)
	for _, listed := range *DecodeJSON(t, resp, &[]*api.Project{}) {
		assert.Equal(t, api.StateOpen, listed.State)
		listedIDs = append(listedIDs, listed.ID)
	}
	assert.Contains(t, listedIDs, project.ID, "created project must appear in the scope's list")
	assert.IsDecreasing(t, listedIDs, "list must be ordered so pagination is stable")

	req = NewRequest(t, "GET", projectURL).AddTokenAuth(token)
	assert.Equal(t, project.ID, DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.Project{}).ID)

	newTitle, closed := "renamed", api.StateClosed
	req = NewRequestWithJSON(t, "PATCH", projectURL, &api.EditProjectOption{Title: &newTitle, State: &closed}).AddTokenAuth(token)
	updated := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.Project{})
	assert.Equal(t, newTitle, updated.Title)
	assert.Equal(t, api.StateClosed, updated.State)
	assert.NotNil(t, updated.ClosedAt)

	// a closed project is read-only
	req = NewRequestWithJSON(t, "POST", projectURL+"/columns", &api.CreateProjectColumnOption{Title: "nope"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	req = NewRequestWithJSON(t, "PATCH", projectURL, map[string]string{"title": ""}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: project.ID, Title: newTitle})

	open := api.StateOpen
	req = NewRequestWithJSON(t, "PATCH", projectURL, &api.EditProjectOption{State: &open}).AddTokenAuth(token)
	reopened := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.Project{})
	assert.Equal(t, api.StateOpen, reopened.State)
	assert.Nil(t, reopened.ClosedAt, "reopening must clear closed_at")

	columnIDs := make([]int64, 0, 2)
	for _, title := range []string{"todo", "doing"} {
		req = NewRequestWithJSON(t, "POST", projectURL+"/columns", &api.CreateProjectColumnOption{Title: title, Color: "#FF5733"}).AddTokenAuth(token)
		column := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.ProjectColumn{})
		assert.Equal(t, title, column.Title)
		assert.Equal(t, "#FF5733", column.Color)
		columnIDs = append(columnIDs, column.ID)
	}

	req = NewRequestWithJSON(t, "POST", projectURL+"/columns", &api.CreateProjectColumnOption{Title: "bad", Color: "red"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequest(t, "GET", projectURL+"/columns").AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	allColumns := *DecodeJSON(t, resp, &[]*api.ProjectColumn{})
	// basic_kanban seeds columns of its own, so only assert on the two just added
	require.GreaterOrEqual(t, len(allColumns), 2)
	assert.Equal(t, strconv.Itoa(len(allColumns)), resp.Header().Get("X-Total-Count"))

	columnURL := fmt.Sprintf("%s/columns/%d", projectURL, columnIDs[0])
	req = NewRequest(t, "GET", columnURL).AddTokenAuth(token)
	assert.Equal(t, "todo", DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ProjectColumn{}).Title)

	renamed, sorting := "todo!", 3
	req = NewRequestWithJSON(t, "PATCH", columnURL, &api.EditProjectColumnOption{Title: &renamed, Sorting: &sorting}).AddTokenAuth(token)
	editedColumn := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ProjectColumn{})
	assert.Equal(t, renamed, editedColumn.Title)
	assert.Equal(t, sorting, editedColumn.Sorting)

	tooLarge := 1000
	req = NewRequestWithJSON(t, "PATCH", columnURL, &api.EditProjectColumnOption{Sorting: &tooLarge}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// 0 is in range and is how a column moves first, so it must reach the database
	zero := 0
	req = NewRequestWithJSON(t, "PATCH", columnURL, &api.EditProjectColumnOption{Sorting: &zero}).AddTokenAuth(token)
	assert.Equal(t, 0, DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ProjectColumn{}).Sorting)
	assert.EqualValues(t, 0, unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: columnIDs[0]}).Sorting)

	req = NewRequestWithJSON(t, "PATCH", columnURL, map[string]string{"title": ""}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequest(t, "POST", columnURL+"/default").AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: columnIDs[0], Default: true})

	reversed := make([]int64, 0, len(allColumns))
	for _, column := range slices.Backward(allColumns) {
		reversed = append(reversed, column.ID)
	}
	req = NewRequestWithJSON(t, "POST", projectURL+"/columns/move", &api.MoveProjectColumnsOption{ColumnIDs: reversed}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", projectURL+"/columns").AddTokenAuth(token)
	reordered := *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]*api.ProjectColumn{})
	gotOrder := make([]int64, 0, len(reordered))
	for _, column := range reordered {
		gotOrder = append(gotOrder, column.ID)
	}
	assert.Equal(t, reversed, gotOrder)

	// a partial list would silently drop columns, so it must be rejected
	req = NewRequestWithJSON(t, "POST", projectURL+"/columns/move", &api.MoveProjectColumnsOption{ColumnIDs: reversed[:1]}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// a non-default column forces assign *and* move, the path that writes the timeline entry
	req = NewRequest(t, "POST", fmt.Sprintf("%s/columns/%d/issues/%d", projectURL, columnIDs[1], scope.issueID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
	unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{
		ProjectID: project.ID, IssueID: scope.issueID, ProjectColumnID: columnIDs[1],
	})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		Type: issues_model.CommentTypeProjectColumn, IssueID: scope.issueID, ProjectID: project.ID,
	})

	req = NewRequest(t, "GET", fmt.Sprintf("%s/columns/%d/issues", projectURL, columnIDs[1])).AddTokenAuth(token)
	issues := *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]api.Issue{})
	require.Len(t, issues, 1)
	assert.Equal(t, scope.issueID, issues[0].ID)

	// the sibling column must not report the same issue
	req = NewRequest(t, "GET", fmt.Sprintf("%s/columns/%d/issues", projectURL, columnIDs[0])).AddTokenAuth(token)
	assert.Empty(t, *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]api.Issue{}))

	// an issue that is not in the project cannot be moved within it
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("%s/issues/%d/move", projectURL, scope.foreignIssueID),
		&api.MoveProjectIssueOption{ColumnID: columnIDs[0]}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	sortPos := int64(7)
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("%s/issues/%d/move", projectURL, scope.issueID),
		&api.MoveProjectIssueOption{ColumnID: columnIDs[0], Sorting: &sortPos}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	moved := unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{ProjectID: project.ID, IssueID: scope.issueID})
	assert.Equal(t, columnIDs[0], moved.ProjectColumnID)
	assert.Equal(t, sortPos, moved.Sorting)

	// removing through a column the issue no longer occupies must not detach it
	req = NewRequest(t, "DELETE", fmt.Sprintf("%s/columns/%d/issues/%d", projectURL, columnIDs[1], scope.issueID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
	unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{ProjectID: project.ID, IssueID: scope.issueID})

	req = NewRequest(t, "DELETE", fmt.Sprintf("%s/columns/%d/issues/%d", projectURL, columnIDs[0], scope.issueID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &project_model.ProjectIssue{ProjectID: project.ID, IssueID: scope.issueID})

	// the default column cannot go while it is still the landing column
	req = NewRequest(t, "DELETE", columnURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	secondColumnURL := fmt.Sprintf("%s/columns/%d", projectURL, columnIDs[1])
	req = NewRequest(t, "DELETE", secondColumnURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", secondColumnURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "DELETE", projectURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", projectURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func testAPIProjectPermissions(t *testing.T, ownerToken, outsiderToken string) {
	// fixture project 1 belongs to repo1, so this needs no project of its own
	const projectURL = "/api/v1/repos/user2/repo1/projects/1"

	req := NewRequestWithJSON(t, "PATCH", projectURL, &api.EditProjectOption{Title: new("hijacked")}).AddTokenAuth(outsiderToken)
	MakeRequest(t, req, http.StatusForbidden)

	MakeRequest(t, NewRequest(t, "DELETE", projectURL).AddTokenAuth(outsiderToken), http.StatusForbidden)

	// a project ID from another owner must not be reachable through this repo's path
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/projects/4").AddTokenAuth(ownerToken), http.StatusNotFound)
}

// testAPIProjectVisibility pins the permission boundary of the listing routes: a private
// organization's boards must stay invisible to outsiders, and a signed-in user with plain
// repository read access must not see fewer issues than an anonymous one.
func testAPIProjectVisibility(t *testing.T, outsider string) {
	for _, url := range []string{"/api/v1/orgs/privated_org/projects", "/api/v1/users/privated_org/projects"} {
		MakeRequest(t, NewRequest(t, "GET", url).AddTokenAuth(outsider), http.StatusNotFound)
	}

	// column 2 of repo1's public board holds issue 3
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/projects/1/columns/2/issues").AddTokenAuth(outsider)
	assert.NotEmpty(t, *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]api.Issue{}))

	// user1 is on private_org35's Owners team, so permissions are not what must deny here
	publicOnly := getUserToken(t, "user1", auth_model.AccessTokenScopeReadUser,
		auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadIssue,
		auth_model.AccessTokenScopePublicOnly)
	for _, url := range []string{"/api/v1/orgs/private_org35/projects", "/api/v1/users/private_org35/projects"} {
		MakeRequest(t, NewRequest(t, "GET", url).AddTokenAuth(publicOnly), http.StatusForbidden)
	}

	// a public org's board is readable by an outsider but not writable
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/orgs/org3/projects").AddTokenAuth(outsider), http.StatusOK)
	req = NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/projects", &api.CreateProjectOption{Title: "outsider"}).AddTokenAuth(outsider)
	MakeRequest(t, req, http.StatusNotFound)
}

// testAPIRepoProjectsModeOwner pins the API to the same Projects-unit mode the web honours:
// with repo-level boards switched off the UI 404s, so the API must not keep serving them.
func testAPIRepoProjectsModeOwner(t *testing.T, token string) {
	hasProjects, ownerMode := true, "owner"
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1", &api.EditRepoOption{
		HasProjects: &hasProjects, ProjectsMode: &ownerMode,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/projects").AddTokenAuth(token), http.StatusNotFound)
	req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/projects", &api.CreateProjectOption{Title: "hidden"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// restore the fixture mode, so this subtest does not constrain sibling ordering
	allMode := "all"
	req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1", &api.EditRepoOption{
		HasProjects: &hasProjects, ProjectsMode: &allMode,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
}
