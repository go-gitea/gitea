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
