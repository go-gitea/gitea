package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowConcurrency_NoCancellation(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-download-task-logs", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/variables/qwe", user2.Name, repo.Name), &api.CreateVariableOption{
				Value: "abc123",
			}).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		wf1TreePath := ".gitea/workflows/concurrent-workflow-1.yml"
		wf1FileContent := `name: concurrent-workflow-1
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-1.yml'
concurrency:
  group: workflow-main-abc123
jobs:
  wf1-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job from workflow1'
`
		wf2TreePath := ".gitea/workflows/concurrent-workflow-2.yml"
		wf2FileContent := `name: concurrent-workflow-2
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-2.yml'
concurrency:
  group: workflow-${{ github.ref_name }}-${{ vars.qwe }}
jobs:
  wf2-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job from workflow2'
`
		wf3TreePath := ".gitea/workflows/concurrent-workflow-3.yml"
		wf3FileContent := `name: concurrent-workflow-3
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-3.yml'
concurrency:
  group: workflow-main-abc${{ 123 }}
jobs:
  wf3-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job from workflow3'
`
		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)
		opts2 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf2TreePath), wf2FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf2TreePath, opts2)
		opts3 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf3TreePath), wf3FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf3TreePath, opts3)

		// fetch and exec workflow1, workflow2 and workflow3 are blocked
		task := runner.fetchTask(t)
		actionTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.Id})
		actionRunJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask.JobID})
		actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: actionRunJob.RunID})
		assert.Equal(t, "workflow-main-abc123", actionRun.ConcurrencyGroup)
		assert.Equal(t, "concurrent-workflow-1.yml", actionRun.WorkflowID)
		runner.fetchNoTask(t)
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// fetch workflow2 or workflow3
		workflowNames := []string{"concurrent-workflow-2.yml", "concurrent-workflow-3.yml"}
		task = runner.fetchTask(t)
		actionTask = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.Id})
		actionRunJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask.JobID})
		actionRun = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: actionRunJob.RunID})
		assert.Contains(t, workflowNames, actionRun.WorkflowID)
		workflowNames = slices.DeleteFunc(workflowNames, func(wfn string) bool { return wfn == actionRun.WorkflowID })
		assert.Equal(t, "workflow-main-abc123", actionRun.ConcurrencyGroup)
		runner.fetchNoTask(t)
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// fetch the last workflow (workflow2 or workflow3)
		task = runner.fetchTask(t)
		actionTask = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.Id})
		actionRunJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask.JobID})
		actionRun = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: actionRunJob.RunID})
		assert.Equal(t, "workflow-main-abc123", actionRun.ConcurrencyGroup)
		assert.Equal(t, workflowNames[0], actionRun.WorkflowID)
		runner.fetchNoTask(t)
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		doAPIDeleteRepository(httpContext)(t)
	})
}
