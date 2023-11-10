// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	secret_model "code.gitea.io/gitea/models/secret"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	secret_module "code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func pickTask(ctx context.Context, runner *actions_model.ActionRunner) (*runnerv1.Task, bool, error) {
	t, ok, err := actions_model.CreateTaskForRunner(ctx, runner)
	if err != nil {
		return nil, false, fmt.Errorf("CreateTaskForRunner: %w", err)
	}
	if !ok {
		return nil, false, nil
	}

	actions.CreateCommitStatus(ctx, t.Job)

	task := &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: t.Job.WorkflowPayload,
		Context:         generateTaskContext(t),
		Secrets:         getSecretsOfTask(ctx, t),
		Vars:            getVariablesOfTask(ctx, t),
	}

	if needs, err := findTaskNeeds(ctx, t); err != nil {
		log.Error("Cannot find needs for task %v: %v", t.ID, err)
		// Go on with empty needs.
		// If return error, the task will be wild, which means the runner will never get it when it has been assigned to the runner.
		// In contrast, missing needs is less serious.
		// And the task will fail and the runner will report the error in the logs.
	} else {
		task.Needs = needs
	}

	return task, true, nil
}

func getSecretsOfTask(ctx context.Context, task *actions_model.ActionTask) map[string]string {
	secrets := map[string]string{}

	secrets["GITHUB_TOKEN"] = task.Token
	secrets["GITEA_TOKEN"] = task.Token

	if task.Job.Run.IsForkPullRequest && task.Job.Run.TriggerEvent != actions_module.GithubEventPullRequestTarget {
		// ignore secrets for fork pull request, except GITHUB_TOKEN and GITEA_TOKEN which are automatically generated.
		// for the tasks triggered by pull_request_target event, they could access the secrets because they will run in the context of the base branch
		// see the documentation: https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_target
		return secrets
	}

	ownerSecrets, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{OwnerID: task.Job.Run.Repo.OwnerID})
	if err != nil {
		log.Error("find secrets of owner %v: %v", task.Job.Run.Repo.OwnerID, err)
		// go on
	}
	repoSecrets, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{RepoID: task.Job.Run.RepoID})
	if err != nil {
		log.Error("find secrets of repo %v: %v", task.Job.Run.RepoID, err)
		// go on
	}

	for _, secret := range append(ownerSecrets, repoSecrets...) {
		if v, err := secret_module.DecryptSecret(setting.SecretKey, secret.Data); err != nil {
			log.Error("decrypt secret %v %q: %v", secret.ID, secret.Name, err)
			// go on
		} else {
			secrets[secret.Name] = v
		}
	}

	return secrets
}

func getVariablesOfTask(ctx context.Context, task *actions_model.ActionTask) map[string]string {
	variables := map[string]string{}

	// Org / User level
	ownerVariables, err := actions_model.FindVariables(ctx, actions_model.FindVariablesOpts{OwnerID: task.Job.Run.Repo.OwnerID})
	if err != nil {
		log.Error("find variables of org: %d, error: %v", task.Job.Run.Repo.OwnerID, err)
	}

	// Repo level
	repoVariables, err := actions_model.FindVariables(ctx, actions_model.FindVariablesOpts{RepoID: task.Job.Run.RepoID})
	if err != nil {
		log.Error("find variables of repo: %d, error: %v", task.Job.Run.RepoID, err)
	}

	// Level precedence: Repo > Org / User
	for _, v := range append(ownerVariables, repoVariables...) {
		variables[v.Name] = v.Data
	}

	return variables
}

func generateTaskContext(t *actions_model.ActionTask) *structpb.Struct {
	event := map[string]any{}
	_ = json.Unmarshal([]byte(t.Job.Run.EventPayload), &event)

	// TriggerEvent is added in https://github.com/go-gitea/gitea/pull/25229
	// This fallback is for the old ActionRun that doesn't have the TriggerEvent field
	// and should be removed in 1.22
	eventName := t.Job.Run.TriggerEvent
	if eventName == "" {
		eventName = t.Job.Run.Event.Event()
	}

	baseRef := ""
	headRef := ""
	ref := t.Job.Run.Ref
	sha := t.Job.Run.CommitSHA
	if pullPayload, err := t.Job.Run.GetPullRequestEventPayload(); err == nil && pullPayload.PullRequest != nil && pullPayload.PullRequest.Base != nil && pullPayload.PullRequest.Head != nil {
		baseRef = pullPayload.PullRequest.Base.Ref
		headRef = pullPayload.PullRequest.Head.Ref

		// if the TriggerEvent is pull_request_target, ref and sha need to be set according to the base of pull request
		// In GitHub's documentation, ref should be the branch or tag that triggered workflow. But when the TriggerEvent is pull_request_target,
		// the ref will be the base branch.
		if t.Job.Run.TriggerEvent == actions_module.GithubEventPullRequestTarget {
			ref = git.BranchPrefix + pullPayload.PullRequest.Base.Name
			sha = pullPayload.PullRequest.Base.Sha
		}
	}

	refName := git.RefName(ref)

	taskContext, err := structpb.NewStruct(map[string]any{
		// standard contexts, see https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
		"action":            "",                                                   // string, The name of the action currently running, or the id of a step. GitHub removes special characters, and uses the name __run when the current step runs a script without an id. If you use the same action more than once in the same job, the name will include a suffix with the sequence number with underscore before it. For example, the first script you run will have the name __run, and the second script will be named __run_2. Similarly, the second invocation of actions/checkout will be actionscheckout2.
		"action_path":       "",                                                   // string, The path where an action is located. This property is only supported in composite actions. You can use this path to access files located in the same repository as the action.
		"action_ref":        "",                                                   // string, For a step executing an action, this is the ref of the action being executed. For example, v2.
		"action_repository": "",                                                   // string, For a step executing an action, this is the owner and repository name of the action. For example, actions/checkout.
		"action_status":     "",                                                   // string, For a composite action, the current result of the composite action.
		"actor":             t.Job.Run.TriggerUser.Name,                           // string, The username of the user that triggered the initial workflow run. If the workflow run is a re-run, this value may differ from github.triggering_actor. Any workflow re-runs will use the privileges of github.actor, even if the actor initiating the re-run (github.triggering_actor) has different privileges.
		"api_url":           setting.AppURL + "api/v1",                            // string, The URL of the GitHub REST API.
		"base_ref":          baseRef,                                              // string, The base_ref or target branch of the pull request in a workflow run. This property is only available when the event that triggers a workflow run is either pull_request or pull_request_target.
		"env":               "",                                                   // string, Path on the runner to the file that sets environment variables from workflow commands. This file is unique to the current step and is a different file for each step in a job. For more information, see "Workflow commands for GitHub Actions."
		"event":             event,                                                // object, The full event webhook payload. You can access individual properties of the event using this context. This object is identical to the webhook payload of the event that triggered the workflow run, and is different for each event. The webhooks for each GitHub Actions event is linked in "Events that trigger workflows." For example, for a workflow run triggered by the push event, this object contains the contents of the push webhook payload.
		"event_name":        eventName,                                            // string, The name of the event that triggered the workflow run.
		"event_path":        "",                                                   // string, The path to the file on the runner that contains the full event webhook payload.
		"graphql_url":       "",                                                   // string, The URL of the GitHub GraphQL API.
		"head_ref":          headRef,                                              // string, The head_ref or source branch of the pull request in a workflow run. This property is only available when the event that triggers a workflow run is either pull_request or pull_request_target.
		"job":               fmt.Sprint(t.JobID),                                  // string, The job_id of the current job.
		"ref":               ref,                                                  // string, The fully-formed ref of the branch or tag that triggered the workflow run. For workflows triggered by push, this is the branch or tag ref that was pushed. For workflows triggered by pull_request, this is the pull request merge branch. For workflows triggered by release, this is the release tag created. For other triggers, this is the branch or tag ref that triggered the workflow run. This is only set if a branch or tag is available for the event type. The ref given is fully-formed, meaning that for branches the format is refs/heads/<branch_name>, for pull requests it is refs/pull/<pr_number>/merge, and for tags it is refs/tags/<tag_name>. For example, refs/heads/feature-branch-1.
		"ref_name":          refName.ShortName(),                                  // string, The short ref name of the branch or tag that triggered the workflow run. This value matches the branch or tag name shown on GitHub. For example, feature-branch-1.
		"ref_protected":     false,                                                // boolean, true if branch protections are configured for the ref that triggered the workflow run.
		"ref_type":          refName.RefType(),                                    // string, The type of ref that triggered the workflow run. Valid values are branch or tag.
		"path":              "",                                                   // string, Path on the runner to the file that sets system PATH variables from workflow commands. This file is unique to the current step and is a different file for each step in a job. For more information, see "Workflow commands for GitHub Actions."
		"repository":        t.Job.Run.Repo.OwnerName + "/" + t.Job.Run.Repo.Name, // string, The owner and repository name. For example, Codertocat/Hello-World.
		"repository_owner":  t.Job.Run.Repo.OwnerName,                             // string, The repository owner's name. For example, Codertocat.
		"repositoryUrl":     t.Job.Run.Repo.HTMLURL(),                             // string, The Git URL to the repository. For example, git://github.com/codertocat/hello-world.git.
		"retention_days":    "",                                                   // string, The number of days that workflow run logs and artifacts are kept.
		"run_id":            fmt.Sprint(t.Job.RunID),                              // string, A unique number for each workflow run within a repository. This number does not change if you re-run the workflow run.
		"run_number":        fmt.Sprint(t.Job.Run.Index),                          // string, A unique number for each run of a particular workflow in a repository. This number begins at 1 for the workflow's first run, and increments with each new run. This number does not change if you re-run the workflow run.
		"run_attempt":       fmt.Sprint(t.Job.Attempt),                            // string, A unique number for each attempt of a particular workflow run in a repository. This number begins at 1 for the workflow run's first attempt, and increments with each re-run.
		"secret_source":     "Actions",                                            // string, The source of a secret used in a workflow. Possible values are None, Actions, Dependabot, or Codespaces.
		"server_url":        setting.AppURL,                                       // string, The URL of the GitHub server. For example: https://github.com.
		"sha":               sha,                                                  // string, The commit SHA that triggered the workflow. The value of this commit SHA depends on the event that triggered the workflow. For more information, see "Events that trigger workflows." For example, ffac537e6cbbf934b08745a378932722df287a53.
		"token":             t.Token,                                              // string, A token to authenticate on behalf of the GitHub App installed on your repository. This is functionally equivalent to the GITHUB_TOKEN secret. For more information, see "Automatic token authentication."
		"triggering_actor":  "",                                                   // string, The username of the user that initiated the workflow run. If the workflow run is a re-run, this value may differ from github.actor. Any workflow re-runs will use the privileges of github.actor, even if the actor initiating the re-run (github.triggering_actor) has different privileges.
		"workflow":          t.Job.Run.WorkflowID,                                 // string, The name of the workflow. If the workflow file doesn't specify a name, the value of this property is the full path of the workflow file in the repository.
		"workspace":         "",                                                   // string, The default working directory on the runner for steps, and the default location of your repository when using the checkout action.

		// additional contexts
		"gitea_default_actions_url": setting.Actions.DefaultActionsURL.URL(),
	})
	if err != nil {
		log.Error("structpb.NewStruct failed: %v", err)
	}

	return taskContext
}

func findTaskNeeds(ctx context.Context, task *actions_model.ActionTask) (map[string]*runnerv1.TaskNeed, error) {
	if err := task.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("LoadAttributes: %w", err)
	}
	if len(task.Job.Needs) == 0 {
		return nil, nil
	}
	needs := container.SetOf(task.Job.Needs...)

	jobs, _, err := actions_model.FindRunJobs(ctx, actions_model.FindRunJobOptions{RunID: task.Job.RunID})
	if err != nil {
		return nil, fmt.Errorf("FindRunJobs: %w", err)
	}

	ret := make(map[string]*runnerv1.TaskNeed, len(needs))
	for _, job := range jobs {
		if !needs.Contains(job.JobID) {
			continue
		}
		if job.TaskID == 0 || !job.Status.IsDone() {
			// it shouldn't happen, or the job has been rerun
			continue
		}
		outputs := make(map[string]string)
		got, err := actions_model.FindTaskOutputByTaskID(ctx, job.TaskID)
		if err != nil {
			return nil, fmt.Errorf("FindTaskOutputByTaskID: %w", err)
		}
		for _, v := range got {
			outputs[v.OutputKey] = v.OutputValue
		}
		ret[job.JobID] = &runnerv1.TaskNeed{
			Outputs: outputs,
			Result:  runnerv1.Result(job.Status),
		}
	}

	return ret, nil
}
