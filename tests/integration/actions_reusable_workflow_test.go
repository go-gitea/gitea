package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestJobUsesReusableWorkflow(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, user2Token, "workflow-call-test", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

		defaultRunner := newMockRunner()
		defaultRunner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// add a variable for test
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/variables/myvar", repo.OwnerName, repo.Name), &api.CreateVariableOption{
				Value: "abc123",
			}).
			AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusCreated)
		// add a secret for test
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/actions/secrets/mysecret", repo.OwnerName, repo.Name), api.CreateOrUpdateSecretOption{
			Data: "secRET-t0Ken",
		}).AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusCreated)

		createRepoWorkflowFile(t, user2, repo, ".gitea/workflows/reusable1.yaml",
			`name: Reusable1
on:
  workflow_call:
    inputs:
      str_input:
        type: string
      num_input:
       type: number
      bool_input:
       type: boolean
      parent_var:
        type: string
    secrets:
      parent_token:

jobs:
  r1-job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'reusable1-job1'
`)

		createRepoWorkflowFile(t, user2, repo, ".gitea/workflows/caller.yaml",
			`name: Caller
on:
  push:
    paths:
      - '.gitea/workflows/caller.yaml'
jobs:
  caller-job1:
    uses: './.gitea/workflows/reusable1.yaml'
    with:
      str_input: 'from caller job1'
      num_input: ${{ 2.3e2 }}
      bool_input: ${{ gitea.event_name == 'push' }}
      parent_var: ${{ vars.myvar }}
    secrets:
      parent_token: ${{ secrets.mysecret }}
`)

		task1 := defaultRunner.fetchTask(t)
		_, job, _ := getTaskAndJobAndRunByTaskID(t, task1.Id)
		assert.Equal(t, "r1-job1", job.JobID)
		eventJSON, err := task1.GetContext().Fields["event"].GetStructValue().MarshalJSON()
		assert.NoError(t, err)
		var payload api.WorkflowCallPayload
		assert.NoError(t, json.Unmarshal(eventJSON, &payload))
		if assert.Len(t, payload.Inputs, 4) {
			assert.Equal(t, "from caller job1", payload.Inputs["str_input"])
			assert.EqualValues(t, 230, payload.Inputs["num_input"])
			assert.Equal(t, true, payload.Inputs["bool_input"])
			assert.Equal(t, "abc123", payload.Inputs["parent_var"])
		}
		if assert.Len(t, task1.Secrets, 3) {
			assert.Contains(t, task1.Secrets, "GITEA_TOKEN")
			assert.Contains(t, task1.Secrets, "GITHUB_TOKEN")
			assert.Equal(t, "secRET-t0Ken", task1.Secrets["parent_token"])
		}
	})
}

func createRepoWorkflowFile(t *testing.T, u *user_model.User, repo *repo_model.Repository, treePath, content string) {
	token := getTokenForLoggedInUser(t, loginUser(t, u.Name), auth_model.AccessTokenScopeWriteRepository)
	opts := getWorkflowCreateFileOptions(u, repo.DefaultBranch, "create "+treePath, content)
	createWorkflowFile(t, token, repo.OwnerName, repo.Name, treePath, opts)
}
