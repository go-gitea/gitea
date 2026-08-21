// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"net/http"
	"strconv"
	"testing"

	org_model "gitea.dev/models/organization"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/test"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanWriteProjectWorkflows(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("repo owner can write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, 1)

		assert.True(t, canWriteProjectWorkflows(ctx, &project_model.Project{Type: project_model.TypeRepository}))
	})

	t.Run("repo reader cannot write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")
		contexttest.LoadRepo(t, ctx, 1)

		assert.False(t, canWriteProjectWorkflows(ctx, &project_model.Project{Type: project_model.TypeRepository}))
	})

	t.Run("individual owner can write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/user2/-/projects/4/workflows")
		contexttest.LoadUser(t, ctx, 2)
		ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		assert.True(t, canWriteProjectWorkflows(ctx, &project_model.Project{Type: project_model.TypeIndividual}))
	})

	t.Run("individual reader cannot write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/user2/-/projects/4/workflows")
		contexttest.LoadUser(t, ctx, 1)
		ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		assert.False(t, canWriteProjectWorkflows(ctx, &project_model.Project{Type: project_model.TypeIndividual}))
	})

	t.Run("org owner can write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/org3/-/projects/4/workflows")
		contexttest.LoadUser(t, ctx, 2)
		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{ID: 3})
		ctx.ContextUser = org.AsUser()
		ctx.Org.Organization = org

		assert.True(t, canWriteProjectWorkflows(ctx, &project_model.Project{Type: project_model.TypeOrganization}))
	})

	t.Run("org visitor cannot write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/org3/-/projects/4/workflows")
		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{ID: 3})
		ctx.ContextUser = org.AsUser()
		ctx.Org.Organization = org

		assert.False(t, canWriteProjectWorkflows(ctx, &project_model.Project{Type: project_model.TypeOrganization}))
	})
}

func TestWorkflowsRepoPageSetsCanWriteProjects(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("repo owner sees writable workflow page", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, 1)
		ctx.SetPathParam("id", "1")

		Workflows(ctx)

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, true, ctx.Data["CanWriteProjects"])
	})

	t.Run("repo reader sees readonly workflow page", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")
		contexttest.LoadRepo(t, ctx, 1)
		ctx.SetPathParam("id", "1")

		Workflows(ctx)

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, false, ctx.Data["CanWriteProjects"])
	})
}

func TestWorkflowsReadEndpoints(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("events endpoint stays readable for repo readers", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows/events")
		contexttest.LoadRepo(t, ctx, 1)
		ctx.SetPathParam("id", "1")

		WorkflowsEvents(ctx)

		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("options endpoint stays readable for repo readers", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows/options")
		contexttest.LoadRepo(t, ctx, 1)
		ctx.SetPathParam("id", "1")

		WorkflowsOptions(ctx)

		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestPrepareProjectRouteScope(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("repo route rejects an owner-scoped project owned by the repo's owner", func(t *testing.T) {
		// regression test: prepareProject used to key off p.OwnerID instead of the route, and
		// context.RepoAssignment sets ctx.ContextUser to the repo owner, so an owner-level project
		// owned by that same user used to leak through when requested via a repo route.
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/999/workflows")
		contexttest.LoadRepo(t, ctx, 1) // repo1 is owned by user2
		ctx.ContextUser = ctx.Repo.Owner

		ownerProject := project_model.Project{
			Title:   "owner-scoped project",
			OwnerID: ctx.ContextUser.ID,
			Type:    project_model.TypeIndividual,
		}
		require.NoError(t, project_model.NewProject(ctx, &ownerProject))
		t.Cleanup(func() {
			require.NoError(t, project_model.DeleteProjectByID(ctx, ownerProject.ID))
		})
		ctx.SetPathParam("id", strconv.FormatInt(ownerProject.ID, 10))

		p := prepareProject(ctx)

		assert.Nil(t, p)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("owner route rejects a repository-scoped project", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/-/projects/1/workflows")
		contexttest.LoadUser(t, ctx, 2)
		ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		ctx.SetPathParam("id", "1") // project 1 is a repository-scoped project on repo1

		p := prepareProject(ctx)

		assert.Nil(t, p)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("owner route accepts a matching owner-scoped project", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/user2/-/projects/1/workflows")
		contexttest.LoadUser(t, ctx, 2)
		ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		ownerProject := project_model.Project{
			Title:   "owner-scoped project",
			OwnerID: ctx.ContextUser.ID,
			Type:    project_model.TypeIndividual,
		}
		require.NoError(t, project_model.NewProject(ctx, &ownerProject))
		t.Cleanup(func() {
			require.NoError(t, project_model.DeleteProjectByID(ctx, ownerProject.ID))
		})
		ctx.SetPathParam("id", strconv.FormatInt(ownerProject.ID, 10))

		p := prepareProject(ctx)

		require.NotNil(t, p)
		assert.Equal(t, ownerProject.ID, p.ID)
	})
}

func TestConvertFormToFiltersRejectsInvalidInput(t *testing.T) {
	unittest.PrepareTestEnv(t)
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1}) // repo1's project

	t.Run("unknown issue_type is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToFilters(ctx, project, project_model.WorkflowEventItemColumnChanged, map[string]any{
			"issue_type": "not_a_real_type",
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		msg := test.ParseJSONError(resp.Body.Bytes()).ErrorMessage
		assert.Contains(t, msg, "issue_type")
		assert.Contains(t, msg, "not_a_real_type")
	})

	t.Run("non-numeric source_column is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToFilters(ctx, project, project_model.WorkflowEventItemColumnChanged, map[string]any{
			"source_column": "not-a-number",
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		msg := test.ParseJSONError(resp.Body.Bytes()).ErrorMessage
		assert.Contains(t, msg, "source_column")
		assert.Contains(t, msg, "not-a-number")
	})

	t.Run("source_column pointing at a column of a different project is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToFilters(ctx, project, project_model.WorkflowEventItemColumnChanged, map[string]any{
			"source_column": "5", // column 5 belongs to project 2, not project 1
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage, "source_column")
	})

	t.Run("label the project cannot use is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToFilters(ctx, project, project_model.WorkflowEventItemColumnChanged, map[string]any{
			"labels": []any{"3"}, // label 3 belongs to org3, not repo1
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		msg := test.ParseJSONError(resp.Body.Bytes()).ErrorMessage
		assert.Contains(t, msg, "label")
		assert.Contains(t, msg, "3")
	})
}

func TestConvertFormToFiltersIsDeterministic(t *testing.T) {
	unittest.PrepareTestEnv(t)
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})
	ctx, _ := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

	// map iteration order is random, so build the same input from a map literal and
	// require the output to always come back in the same (sorted-key) order
	formFilters := map[string]any{
		"target_column": "2",
		"issue_type":    "issue",
		"source_column": "1",
		"labels":        []any{"1"},
	}

	filters, ok := convertFormToFilters(ctx, project, project_model.WorkflowEventItemColumnChanged, formFilters)
	require.True(t, ok)

	gotTypes := make([]string, 0, len(filters))
	for _, f := range filters {
		gotTypes = append(gotTypes, string(f.Type))
	}
	assert.Equal(t, []string{"issue_type", "labels", "source_column", "target_column"}, gotTypes)
}

func TestConvertFormToActionsRejectsInvalidInput(t *testing.T) {
	unittest.PrepareTestEnv(t)
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1}) // repo1's project

	t.Run("unknown issue_state is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToActions(ctx, project, project_model.WorkflowEventItemColumnChanged, map[string]any{
			"issue_state": "paused",
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		msg := test.ParseJSONError(resp.Body.Bytes()).ErrorMessage
		assert.Contains(t, msg, "issue_state")
		assert.Contains(t, msg, "paused")
	})

	t.Run("non-numeric column is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToActions(ctx, project, project_model.WorkflowEventItemOpened, map[string]any{
			"column": "not-a-number",
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage, "column")
	})

	t.Run("label the project cannot use is rejected", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

		_, ok := convertFormToActions(ctx, project, project_model.WorkflowEventItemColumnChanged, map[string]any{
			"add_labels": []any{"3"}, // label 3 belongs to org3, not repo1
		})

		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		msg := test.ParseJSONError(resp.Body.Bytes()).ErrorMessage
		assert.Contains(t, msg, "label")
		assert.Contains(t, msg, "3")
	})
}

func TestConvertFormToActionsIsDeterministic(t *testing.T) {
	unittest.PrepareTestEnv(t)
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})
	ctx, _ := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows")

	formActions := map[string]any{
		"remove_labels": []any{"1"},
		"issue_state":   "close",
		"add_labels":    []any{"1"},
	}

	actions, ok := convertFormToActions(ctx, project, project_model.WorkflowEventItemColumnChanged, formActions)
	require.True(t, ok)

	gotTypes := make([]string, 0, len(actions))
	for _, a := range actions {
		gotTypes = append(gotTypes, string(a.Type))
	}
	assert.Equal(t, []string{"add_labels", "issue_state", "remove_labels"}, gotTypes)
}

func TestWorkflowsStatusRejectsInvalidEnabledValue(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, resp := contexttest.MockContext(t, "/user2/repo1/projects/1/workflows/1/status?enabled=maybe")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)

	wf := &project_model.Workflow{
		ProjectID:     1,
		WorkflowEvent: project_model.WorkflowEventItemOpened,
		WorkflowActions: []project_model.WorkflowAction{
			{Type: project_model.WorkflowActionTypeColumn, Value: "1"},
		},
		Enabled: true,
	}
	// use ctx (not t.Context()) for setup/cleanup: t.Context() is already canceled by the
	// time t.Cleanup functions run, but ctx's underlying context is not test-scoped
	require.NoError(t, project_model.CreateWorkflow(ctx, wf))
	t.Cleanup(func() {
		require.NoError(t, project_model.DeleteWorkflow(ctx, wf.ProjectID, wf.ID))
	})
	ctx.SetPathParam("id", "1")
	ctx.SetPathParam("workflow_id", strconv.FormatInt(wf.ID, 10))

	WorkflowsStatus(ctx)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	msg := test.ParseJSONError(resp.Body.Bytes()).ErrorMessage
	assert.Contains(t, msg, "enabled")
	assert.Contains(t, msg, "maybe")

	// the workflow's enabled state must be unchanged, not silently flipped to disabled
	reloaded, err := project_model.GetWorkflowByProjectAndID(ctx, wf.ProjectID, wf.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.Enabled)
}
