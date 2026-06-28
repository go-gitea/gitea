// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/queue"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/forms"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scopedPushWorkflow = `name: Scoped Push
on: push
jobs:
  scoped-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo scoped
`

const scopedPRWorkflow = `name: Scoped PR
on: pull_request
jobs:
  scoped-pr-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo scoped-pr
`

func TestActionsScopedWorkflows(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// createTestRepo creates an Actions-enabled repo owned by user2, used as a scoped-workflow source or consumer.
		createTestRepo := func(t *testing.T, name string, private bool) *repo_model.Repository {
			apiRepo := createActionsTestRepo(t, user2Token, name, private)
			return unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		}

		// registerUserScopedSource registers `source` as a user-level scoped-workflow source for user2 and marks `required` entry names
		registerUserScopedSource := func(t *testing.T, source *repo_model.Repository, required ...string) {
			addReq := NewRequestWithValues(t, "POST", "/user/settings/actions/scoped-workflows/add",
				map[string]string{"repo_name": source.Name})
			user2Session.MakeRequest(t, addReq, http.StatusOK)
			t.Cleanup(func() {
				removeReq := NewRequestWithValues(t, "POST", "/user/settings/actions/scoped-workflows/remove",
					map[string]string{"repo_id": strconv.FormatInt(source.ID, 10)})
				user2Session.MakeRequest(t, removeReq, http.StatusOK)
			})
			if len(required) > 0 {
				vals := url.Values{"repo_id": {strconv.FormatInt(source.ID, 10)}, "workflow_ids": required, "required_workflow_ids": required}
				for _, id := range required {
					// a pattern that matches the source's scoped check regardless of its `name:` (each test source has one workflow)
					vals.Set("required_patterns["+id+"]", source.FullName()+": * / *")
				}
				reqReq := NewRequestWithURLValues(t, "POST", "/user/settings/actions/scoped-workflows/required", vals)
				user2Session.MakeRequest(t, reqReq, http.StatusOK)
			}
		}

		t.Run("Trigger and run creation", func(t *testing.T) {
			// Registered at INSTANCE level via the admin route (owner/name resolution + OwnerID=0 storage);
			// the trigger->execute->rerun below proves an instance-level source drives a consumer run end-to-end and that a rerun stays scoped.
			adminSession := loginUser(t, "user1")
			source := createTestRepo(t, "sw-trigger-source", false)
			// commit the scoped workflow BEFORE registering so the source's own push does not self-trigger.
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/push.yaml", scopedPushWorkflow)
			adminAdd := NewRequestWithValues(t, "POST", "/-/admin/actions/scoped-workflows/add", map[string]string{"repo_name": source.FullName()})
			adminSession.MakeRequest(t, adminAdd, http.StatusOK)
			t.Cleanup(func() {
				rm := NewRequestWithValues(t, "POST", "/-/admin/actions/scoped-workflows/remove", map[string]string{"repo_id": strconv.FormatInt(source.ID, 10)})
				adminSession.MakeRequest(t, rm, http.StatusOK)
			})
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScopedWorkflowSource{OwnerID: 0, SourceRepoID: source.ID})

			consumer := createTestRepo(t, "sw-trigger-consumer", false)
			runner := newMockRunner()
			runner.registerAsRepoRunner(t, consumer.OwnerName, consumer.Name, "sw-trigger-runner", []string{"ubuntu-latest"}, false)

			createRepoWorkflowFile(t, user2, user2Token, consumer, "marker.txt", "trigger")

			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true})
			assert.Equal(t, source.ID, run.WorkflowRepoID, "content source is the source repo")
			assert.Equal(t, "push.yaml", run.WorkflowID)
			assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID}), "only the scoped run, no repo-level run")
			job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID})

			// runs in the CONSUMER's context and reaches a terminal state
			task := runner.fetchTask(t)
			_, taskJob, taskRun := getTaskAndJobAndRunByTaskID(t, task.Id)
			assert.Equal(t, consumer.ID, taskJob.RepoID)
			assert.Equal(t, run.ID, taskRun.ID)
			runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
			assert.Equal(t, actions_model.StatusSuccess, run.Status)

			// rerun: the rerun is still a scoped run and again executes in the consumer's context
			rerunReq := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", consumer.OwnerName, consumer.Name, run.ID, job.ID))
			user2Session.MakeRequest(t, rerunReq, http.StatusOK)
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{RunID: run.ID, Attempt: 2})
			task2 := runner.fetchTask(t)
			_, taskJob2, taskRun2 := getTaskAndJobAndRunByTaskID(t, task2.Id)
			assert.Equal(t, consumer.ID, taskJob2.RepoID)
			assert.True(t, taskRun2.IsScopedRun, "the rerun is still a scoped run")
			runner.execTask(t, task2, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
			assert.Equal(t, actions_model.StatusSuccess, run.Status)
		})

		t.Run("Opt-out", func(t *testing.T) {
			// opt-out: a consumer can disable a non-required scoped workflow, but a required one cannot be disabled.
			source := createTestRepo(t, "sw-optout-source", false)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/push.yaml", scopedPushWorkflow)
			registerUserScopedSource(t, source) // non-required

			// non-required: the kebab "Disable Workflow" item is an active link; disabling then makes a push produce no scoped run.
			consumer := createTestRepo(t, "sw-optout-consumer", false)
			optBody := user2Session.MakeRequest(t, NewRequest(t, "GET",
				fmt.Sprintf("/%s/%s/actions?workflow=push.yaml&scoped_workflow_source_repo_id=%d", consumer.OwnerName, consumer.Name, source.ID)),
				http.StatusOK).Body.String()
			assert.Contains(t, optBody, "Disable Workflow")
			assert.Contains(t, optBody, "disable?workflow=push.yaml", "non-required scoped workflow: Disable Workflow is a clickable link")
			disableReq := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/disable?workflow=push.yaml&scoped_workflow_source_repo_id=%d", consumer.OwnerName, consumer.Name, source.ID))
			user2Session.MakeRequest(t, disableReq, http.StatusOK)
			createRepoWorkflowFile(t, user2, user2Token, consumer, "marker.txt", "trigger")
			assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true}), "opted-out scoped workflow must not run")

			// required: the kebab "Disable Workflow" item is rendered disabled (no link), and the disable endpoint rejects it.
			reqSource := createTestRepo(t, "sw-optout-req-source", false)
			createRepoWorkflowFile(t, user2, user2Token, reqSource, ".gitea/scoped_workflows/push.yaml", scopedPushWorkflow)
			registerUserScopedSource(t, reqSource, "push.yaml") // required
			reqConsumer := createTestRepo(t, "sw-optout-req-consumer", false)
			requiredBody := user2Session.MakeRequest(t, NewRequest(t, "GET",
				fmt.Sprintf("/%s/%s/actions?workflow=push.yaml&scoped_workflow_source_repo_id=%d", reqConsumer.OwnerName, reqConsumer.Name, reqSource.ID)),
				http.StatusOK).Body.String()
			assert.Contains(t, requiredBody, "Disable Workflow")
			assert.Contains(t, requiredBody, `class="item disabled"`, "required scoped workflow: Disable Workflow is rendered disabled")
			assert.NotContains(t, requiredBody, "disable?workflow=push.yaml", "required scoped workflow: Disable Workflow has no clickable link")
			rejectReq := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/disable?workflow=push.yaml&scoped_workflow_source_repo_id=%d", reqConsumer.OwnerName, reqConsumer.Name, reqSource.ID))
			user2Session.MakeRequest(t, rejectReq, http.StatusBadRequest) // scoped_required_cannot_disable
		})

		t.Run("Local uses resolves to source", func(t *testing.T) {
			// uses: ./ in a scoped workflow resolves against the SOURCE repo, not the consumer.
			// Here the reusable lib lives in the SCOPED workflow dir (allowed by ResolveUses), exercising that path end-to-end.
			source := createTestRepo(t, "sw-uses-source", false)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/lib.yaml", `name: Lib
on:
  workflow_call:
jobs:
  lib_job_source:
    runs-on: ubuntu-latest
    steps:
      - run: echo from-source
`)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/caller.yaml", `name: Caller
on: push
jobs:
  caller_job:
    uses: ./.gitea/scoped_workflows/lib.yaml
`)

			consumer := createTestRepo(t, "sw-uses-consumer", false)
			// a DIFFERENT lib at the same path in the consumer; if uses:./ mis-resolved we would see this job
			createRepoWorkflowFile(t, user2, user2Token, consumer, ".gitea/scoped_workflows/lib.yaml", `name: Lib
on:
  workflow_call:
jobs:
  lib_job_consumer:
    runs-on: ubuntu-latest
    steps:
      - run: echo from-consumer
`)
			// register only AFTER both repos' scoped files exist, so the setup pushes do not trigger
			registerUserScopedSource(t, source)
			createRepoWorkflowFile(t, user2, user2Token, consumer, "marker.txt", "trigger")

			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true, WorkflowID: "caller.yaml"})
			callerJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "caller_job"})
			assert.True(t, callerJob.IsReusableCaller)
			assert.True(t, callerJob.IsExpanded)
			assert.Equal(t, source.ID, callerJob.WorkflowSourceRepoID, "top-level caller's content source is the source repo")
			// the expanded child comes from the SOURCE's lib.yaml, not the consumer's same-path file
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "lib_job_source", ParentJobID: callerJob.ID})
			unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "lib_job_consumer"})
		})

		t.Run("Workflow dispatch", func(t *testing.T) {
			// a scoped on:workflow_dispatch workflow can be triggered manually from the consumer, via both the web form and the API
			source := createTestRepo(t, "sw-dispatch-source", false)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/dispatch.yaml", `name: Scoped Dispatch
on: workflow_dispatch
jobs:
  dispatch-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo dispatch
`)
			registerUserScopedSource(t, source)
			consumer := createTestRepo(t, "sw-dispatch-consumer", false)

			// web form: /run?...&scoped_workflow_source_repo_id=
			webReq := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/run?workflow=dispatch.yaml&scoped_workflow_source_repo_id=%d&ref=refs/heads/%s",
				consumer.OwnerName, consumer.Name, source.ID, consumer.DefaultBranch))
			user2Session.MakeRequest(t, webReq, http.StatusSeeOther)
			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true, WorkflowID: "dispatch.yaml"})
			assert.Equal(t, source.ID, run.WorkflowRepoID, "content source is the source repo")
			assert.NotEmpty(t, run.WorkflowCommitSHA, "scoped dispatch records the source default-branch commit")
			assert.Contains(t, run.Ref, consumer.DefaultBranch, "dispatch runs on the chosen consumer ref")

			// API: /actions/workflows/dispatch.yaml/dispatches?scoped_workflow_source_repo_id=
			apiReq := NewRequestWithURLValues(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/actions/workflows/dispatch.yaml/dispatches?scoped_workflow_source_repo_id=%d", consumer.OwnerName, consumer.Name, source.ID),
				url.Values{"ref": {consumer.DefaultBranch}}).AddTokenAuth(user2Token)
			MakeRequest(t, apiReq, http.StatusNoContent)
			assert.Equal(t, 2, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true, WorkflowID: "dispatch.yaml", Event: "workflow_dispatch"}),
				"both the web form and the API created a scoped dispatch run")
		})

		t.Run("Required scoped check gates the PR merge", func(t *testing.T) {
			// A required scoped workflow's check gates PR merges on a protected branch and cannot be bypassed,
			// whether or not the branch enables its own status check. The scoped check is added to the required set dynamically.
			source := createTestRepo(t, "sw-gate-source", false)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/pr.yaml", scopedPRWorkflow)
			registerUserScopedSource(t, source, "pr.yaml") // required

			// protectAndOpenPR protects consumer's default branch and opens a PR on `branch`, returning a merge-request builder.
			// When statusCheckEnabled it also configures "ci/manual" as the only CONFIGURED required context and satisfies it,
			// so the scoped check is the only thing that can gate the merge; otherwise the rule's own status check stays off.
			protectAndOpenPR := func(t *testing.T, consumer *repo_model.Repository, branch string, statusCheckEnabled bool) func() *RequestWrapper {
				pbValues := map[string]string{
					"rule_name":                  consumer.DefaultBranch,
					"enable_push":                "true",
					"block_admin_merge_override": "true", // otherwise the repo owner bypasses the status check
				}
				if statusCheckEnabled {
					pbValues["enable_status_check"] = "true"
					pbValues["status_check_contexts"] = "ci/manual"
				}
				user2Session.MakeRequest(t, NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/branches/edit", consumer.OwnerName, consumer.Name), pbValues), http.StatusSeeOther)

				prFile := &api.CreateFileOptions{
					FileOptions: api.FileOptions{
						BranchName: consumer.DefaultBranch, NewBranchName: branch, Message: "pr change",
						Author:    api.Identity{Name: user2.Name, Email: user2.Email},
						Committer: api.Identity{Name: user2.Name, Email: user2.Email},
						Dates:     api.CommitDateOptions{Author: time.Now(), Committer: time.Now()},
					},
					ContentBase64: base64.StdEncoding.EncodeToString([]byte("pr change")),
				}
				createWorkflowFile(t, user2Token, consumer.OwnerName, consumer.Name, "pr-change.txt", prFile)
				apiCtx := NewAPITestContext(t, user2.Name, consumer.Name, auth_model.AccessTokenScopeWriteRepository)
				pr, err := doAPICreatePullRequest(apiCtx, consumer.OwnerName, consumer.Name, consumer.DefaultBranch, branch)(t)
				require.NoError(t, err)

				if statusCheckEnabled {
					// satisfy the configured "ci/manual" check so only the scoped check can gate the merge
					manualStatus := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/statuses/%s", consumer.OwnerName, consumer.Name, pr.Head.Sha),
						api.CreateStatusOption{State: commitstatus.CommitStatusSuccess, Context: "ci/manual", TargetURL: "http://test.ci/"}).AddTokenAuth(user2Token)
					user2Session.MakeRequest(t, manualStatus, http.StatusCreated)
				}

				return func() *RequestWrapper {
					return NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", consumer.OwnerName, consumer.Name, pr.Index),
						&forms.MergePullRequestForm{Do: string(repo_model.MergeStyleMerge), MergeMessageField: "merge"}).AddTokenAuth(user2Token)
				}
			}

			t.Run("pending blocks, success allows", func(t *testing.T) {
				consumer := createTestRepo(t, "sw-gate-consumer", false)
				runner := newMockRunner()
				runner.registerAsRepoRunner(t, consumer.OwnerName, consumer.Name, "sw-gate-runner", []string{"ubuntu-latest"}, false)

				mergeReq := protectAndOpenPR(t, consumer, "gate-pr", true)
				run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true})
				assert.Equal(t, source.ID, run.WorkflowRepoID)

				// the pending required scoped check blocks the merge
				assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
				user2Session.MakeRequest(t, mergeReq(), http.StatusMethodNotAllowed)

				// the required scoped run succeeds ->  merge allowed
				task := runner.fetchTask(t)
				runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
				assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
				user2Session.MakeRequest(t, mergeReq(), http.StatusOK)
			})

			t.Run("Actions disabled blocks merge (no bypass)", func(t *testing.T) {
				// must-present: disabling the consumer's Actions unit so the required scoped workflow cannot run.
				// Must BLOCK the merge (the required check is absent), not bypass it.
				consumer := createTestRepo(t, "sw-noact-consumer", false)
				require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), consumer, nil, []unit_model.Type{unit_model.TypeActions}))

				mergeReq := protectAndOpenPR(t, consumer, "noact-pr", true)
				assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true}),
					"Actions disabled, so no scoped run is created")

				// the required scoped check never posted a status ->  must-present blocks the merge (no bypass)
				assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
				user2Session.MakeRequest(t, mergeReq(), http.StatusMethodNotAllowed)
			})

			t.Run("status check disabled: the scoped check still gates", func(t *testing.T) {
				// the scoped check gates the merge even when the branch's OWN status check is off
				consumer := createTestRepo(t, "sw-nocheck-consumer", false)
				runner := newMockRunner()
				runner.registerAsRepoRunner(t, consumer.OwnerName, consumer.Name, "sw-nocheck-runner", []string{"ubuntu-latest"}, false)

				mergeReq := protectAndOpenPR(t, consumer, "nocheck-pr", false)
				unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true})

				// pending scoped check blocks the merge despite the branch's own status check being off
				assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
				user2Session.MakeRequest(t, mergeReq(), http.StatusMethodNotAllowed)

				// the required scoped run succeeds ->  merge allowed
				task := runner.fetchTask(t)
				runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
				assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
				user2Session.MakeRequest(t, mergeReq(), http.StatusOK)
			})
		})

		t.Run("Settings page required patterns", func(t *testing.T) {
			source := createTestRepo(t, "sw-settings-source", false)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/push.yaml", scopedPushWorkflow)
			createRepoWorkflowFile(t, user2, user2Token, source, ".gitea/scoped_workflows/manual.yaml", `name: Manual
on: workflow_dispatch
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`) // workflow_dispatch posts no status -> the settings page must warn instead of listing contexts
			registerUserScopedSource(t, source) // registered; each phase configures it via the /required endpoint
			pattern := source.FullName() + ": * / *"

			setConfigs := func(t *testing.T, vals url.Values) {
				vals.Set("repo_id", strconv.FormatInt(source.ID, 10))
				user2Session.MakeRequest(t, NewRequestWithURLValues(t, "POST", "/user/settings/actions/scoped-workflows/required", vals), http.StatusOK)
			}
			loadSource := func(t *testing.T) *actions_model.ActionScopedWorkflowSource {
				return unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScopedWorkflowSource{OwnerID: user2.ID, SourceRepoID: source.ID})
			}
			settingsBody := func(t *testing.T) string {
				return user2Session.MakeRequest(t, NewRequest(t, "GET", "/user/settings/actions/scoped-workflows"), http.StatusOK).Body.String()
			}

			t.Run("renders the saved pattern and display-name default", func(t *testing.T) {
				setConfigs(t, url.Values{"workflow_ids": {"push.yaml"}, "required_workflow_ids": {"push.yaml"}, "required_patterns[push.yaml]": {pattern}})
				body := settingsBody(t)
				assert.Contains(t, body, `name="required_patterns[push.yaml]"`, "patterns textarea uses the field name the parser expects")
				assert.Contains(t, body, pattern, "the saved pattern round-trips into the textarea")
				// the default prefill must use the workflow display name so it matches the status context the run posts (name: Scoped Push)
				assert.Contains(t, body, `data-default-pattern="`+source.FullName()+`: Scoped Push / *"`)
				// the expected-checks preview derives the exact context a run posts (job scoped-job, event push) for live glob matching
				assert.Contains(t, body, `data-context="`+source.FullName()+`: Scoped Push / scoped-job (push)"`)
			})

			t.Run("live pattern kept as history after un-require", func(t *testing.T) {
				setConfigs(t, url.Values{"workflow_ids": {"push.yaml"}, "required_workflow_ids": {"push.yaml"}, "required_patterns[push.yaml]": {pattern}})
				// un-require: the row still submits workflow_ids + its patterns (the hidden textarea), but not required_workflow_ids
				setConfigs(t, url.Values{"workflow_ids": {"push.yaml"}, "required_patterns[push.yaml]": {pattern}})
				cfg := loadSource(t).WorkflowConfigs["push.yaml"]
				require.NotNil(t, cfg)
				assert.False(t, cfg.Required, "no longer required")
				assert.Equal(t, []string{pattern}, cfg.Patterns, "pattern retained as history")
				assert.Contains(t, settingsBody(t), pattern, "history pattern still rendered, so re-requiring restores it")
			})

			t.Run("orphan config dropped when un-required", func(t *testing.T) {
				// An orphan entry (gone.yaml: required for a file that no longer exists in the source) has no history worth keeping:
				// un-checking Required must drop it entirely, unlike a live un-required workflow.
				setConfigs(t, url.Values{
					"workflow_ids": {"push.yaml", "gone.yaml"}, "required_workflow_ids": {"push.yaml", "gone.yaml"},
					"required_patterns[push.yaml]": {pattern}, "required_patterns[gone.yaml]": {pattern},
				})
				require.True(t, loadSource(t).IsWorkflowRequired("gone.yaml"), "orphan kept while still required")

				// un-require gone.yaml (its row + patterns are still submitted, as the settings page does); push.yaml stays required
				setConfigs(t, url.Values{
					"workflow_ids": {"push.yaml", "gone.yaml"}, "required_workflow_ids": {"push.yaml"},
					"required_patterns[push.yaml]": {pattern}, "required_patterns[gone.yaml]": {pattern},
				})
				src := loadSource(t)
				assert.Nil(t, src.WorkflowConfigs["gone.yaml"], "orphan dropped after un-require, not kept as history")
				assert.True(t, src.IsWorkflowRequired("push.yaml"), "live required workflow kept")
			})

			t.Run("warns when a workflow posts no status checks", func(t *testing.T) {
				// manual.yaml only runs on workflow_dispatch, which posts no commit status: instead of listing expected checks,
				// its row shows a warning not to mark it required (must-present would block forever).
				body := settingsBody(t)
				assert.Contains(t, body, "posts no status checks", "the no-status-check warning is shown")
				assert.NotContains(t, body, `data-context="`+source.FullName()+`: Manual /`, "a workflow_dispatch-only workflow must list no expected contexts")
			})
		})

		t.Run("Distinct sources same filename", func(t *testing.T) {
			// two DIFFERENT source repos with the same filename run independently
			s1 := createTestRepo(t, "sw-multi-s1", false)
			createRepoWorkflowFile(t, user2, user2Token, s1, ".gitea/scoped_workflows/ci.yaml", scopedPushWorkflow)
			s2 := createTestRepo(t, "sw-multi-s2", false)
			createRepoWorkflowFile(t, user2, user2Token, s2, ".gitea/scoped_workflows/ci.yaml", scopedPushWorkflow)
			registerUserScopedSource(t, s1)
			registerUserScopedSource(t, s2)

			consumer := createTestRepo(t, "sw-multi-consumer", false)
			createRepoWorkflowFile(t, user2, user2Token, consumer, "marker.txt", "trigger")

			assert.Equal(t, 2, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true}), "same-named ci.yaml from two sources run independently")
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true, WorkflowRepoID: s1.ID})
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true, WorkflowRepoID: s2.ID})
		})

		t.Run("Detection cache invalidates on source push", func(t *testing.T) {
			// The detection parse is cached per (source, default-branch SHA).
			source := createTestRepo(t, "sw-cache-source", false)
			created := createWorkflowFile(t, user2Token, source.OwnerName, source.Name, ".gitea/scoped_workflows/ci.yaml",
				getWorkflowCreateFileOptions(user2, source.DefaultBranch, "create ci", `name: CI
on: pull_request
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo a
`))
			registerUserScopedSource(t, source)

			consumer := createTestRepo(t, "sw-cache-consumer", false)

			// warm the cache at the source's current SHA: the source triggers on pull_request, so a consumer push is no match
			createRepoWorkflowFile(t, user2, user2Token, consumer, "m1.txt", "trigger")
			assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true}),
				"source triggers on pull_request, so a consumer push must not create a scoped run")

			// switch the source's trigger to push on its default branch
			updateReq := NewRequestWithJSON(t, "PUT",
				fmt.Sprintf("/api/v1/repos/%s/%s/contents/.gitea/scoped_workflows/ci.yaml", source.OwnerName, source.Name),
				&api.UpdateFileOptions{
					SHA:         created.Content.SHA,
					FileOptions: api.FileOptions{BranchName: source.DefaultBranch, Message: "switch to push"},
					ContentBase64: base64.StdEncoding.EncodeToString([]byte(`name: CI
on: push
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo a
`)),
				}).AddTokenAuth(user2Token)
			MakeRequest(t, updateReq, http.StatusOK)

			// the next consumer push must re-detect against the new SHA (on: push) and create the scoped run
			createRepoWorkflowFile(t, user2, user2Token, consumer, "m2.txt", "trigger")
			assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumer.ID, IsScopedRun: true, Event: "push"}),
				"after the source switches to on: push, the next consumer push creates a scoped run")
		})

		t.Run("Deletion cleans up source registration", func(t *testing.T) {
			source := createTestRepo(t, "sw-delete-source", false)

			addReq := NewRequestWithValues(t, "POST", "/user/settings/actions/scoped-workflows/add", map[string]string{"repo_name": source.Name})
			user2Session.MakeRequest(t, addReq, http.StatusOK)
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScopedWorkflowSource{OwnerID: user2.ID, SourceRepoID: source.ID})

			delReq := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s", source.OwnerName, source.Name)).AddTokenAuth(user2Token)
			MakeRequest(t, delReq, http.StatusNoContent)
			unittest.AssertNotExistsBean(t, &actions_model.ActionScopedWorkflowSource{SourceRepoID: source.ID})
		})
	})
}
