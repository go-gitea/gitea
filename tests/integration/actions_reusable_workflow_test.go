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
		// user2 is the owner of actions-reuse-1 repo
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		// create caller repo
		apiRepo1 := createActionsTestRepo(t, user2Token, "actions-reuse-1", false)
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo1.ID})

		defaultRunner := newMockRunner()
		defaultRunner.registerAsRepoRunner(t, repo1.OwnerName, repo1.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// add a variable for test
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/variables/myvar", user2.Name, repo1.Name), &api.CreateVariableOption{
				Value: "abc123",
			}).
			AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusCreated)

		repo1ReusableWorkflowTreePath := ".gitea/workflows/reusable1.yaml"
		repo1ReusableWorkflowFileContent := `name: Reusable1
on:
  workflow_call:
    inputs:
      str_input:
        type: string
      parent_var:
        type: string

jobs:
  r1-job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'reusable1-job1'
`
		reuse1Opts := getWorkflowCreateFileOptions(user2, repo1.DefaultBranch, "create "+repo1ReusableWorkflowTreePath, repo1ReusableWorkflowFileContent)
		createWorkflowFile(t, user2Token, repo1.OwnerName, repo1.Name, repo1ReusableWorkflowTreePath, reuse1Opts)
		callerWorkflowTreePath := ".gitea/workflows/caller.yaml"
		callerWorkflowFileContent := `name: Pull Request
on:
  push:
    paths:
      - '.gitea/workflows/caller.yaml'
jobs:
  caller-job1:
    uses: './.gitea/workflows/reusable1.yaml'
    with:
      str_input: 'from caller job1'
      parent_var: ${{ vars.myvar }}
`
		callerOpts := getWorkflowCreateFileOptions(user2, repo1.DefaultBranch, "create "+callerWorkflowTreePath, callerWorkflowFileContent)
		createWorkflowFile(t, user2Token, repo1.OwnerName, repo1.Name, callerWorkflowTreePath, callerOpts)

		task1 := defaultRunner.fetchTask(t)
		_, job, _ := getTaskAndJobAndRunByTaskID(t, task1.Id)
		assert.Equal(t, "r1-job1", job.JobID)
		eventJSON, err := task1.GetContext().Fields["event"].GetStructValue().MarshalJSON()
		assert.NoError(t, err)
		var payload api.WorkflowCallPayload
		assert.NoError(t, json.Unmarshal(eventJSON, &payload))
		if assert.Len(t, payload.Inputs, 2) {
			assert.Equal(t, "from caller job1", payload.Inputs["str_input"])
			assert.Equal(t, "abc123", payload.Inputs["parent_var"])
		}
	})
}
