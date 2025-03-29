// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
)

// GenerateGiteaContext generate the gitea context without token and gitea_runtime_token
// job can be nil when generating a context for parsing workflow-level expressions
func GenerateGiteaContext(run *actions_model.ActionRun, job *actions_model.ActionRunJob) map[string]any {
	event := map[string]any{}
	_ = json.Unmarshal([]byte(run.EventPayload), &event)

	baseRef := ""
	headRef := ""
	ref := run.Ref
	sha := run.CommitSHA
	if pullPayload, err := run.GetPullRequestEventPayload(); err == nil && pullPayload.PullRequest != nil && pullPayload.PullRequest.Base != nil && pullPayload.PullRequest.Head != nil {
		baseRef = pullPayload.PullRequest.Base.Ref
		headRef = pullPayload.PullRequest.Head.Ref

		// if the TriggerEvent is pull_request_target, ref and sha need to be set according to the base of pull request
		// In GitHub's documentation, ref should be the branch or tag that triggered workflow. But when the TriggerEvent is pull_request_target,
		// the ref will be the base branch.
		if run.TriggerEvent == actions_module.GithubEventPullRequestTarget {
			ref = git.BranchPrefix + pullPayload.PullRequest.Base.Name
			sha = pullPayload.PullRequest.Base.Sha
		}
	}

	refName := git.RefName(ref)

	gitContext := map[string]any{
		// standard contexts, see https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
		"action":            "",                                       // string, The name of the action currently running, or the id of a step. GitHub removes special characters, and uses the name __run when the current step runs a script without an id. If you use the same action more than once in the same job, the name will include a suffix with the sequence number with underscore before it. For example, the first script you run will have the name __run, and the second script will be named __run_2. Similarly, the second invocation of actions/checkout will be actionscheckout2.
		"action_path":       "",                                       // string, The path where an action is located. This property is only supported in composite actions. You can use this path to access files located in the same repository as the action.
		"action_ref":        "",                                       // string, For a step executing an action, this is the ref of the action being executed. For example, v2.
		"action_repository": "",                                       // string, For a step executing an action, this is the owner and repository name of the action. For example, actions/checkout.
		"action_status":     "",                                       // string, For a composite action, the current result of the composite action.
		"actor":             run.TriggerUser.Name,                     // string, The username of the user that triggered the initial workflow run. If the workflow run is a re-run, this value may differ from github.triggering_actor. Any workflow re-runs will use the privileges of github.actor, even if the actor initiating the re-run (github.triggering_actor) has different privileges.
		"api_url":           setting.AppURL + "api/v1",                // string, The URL of the GitHub REST API.
		"base_ref":          baseRef,                                  // string, The base_ref or target branch of the pull request in a workflow run. This property is only available when the event that triggers a workflow run is either pull_request or pull_request_target.
		"env":               "",                                       // string, Path on the runner to the file that sets environment variables from workflow commands. This file is unique to the current step and is a different file for each step in a job. For more information, see "Workflow commands for GitHub Actions."
		"event":             event,                                    // object, The full event webhook payload. You can access individual properties of the event using this context. This object is identical to the webhook payload of the event that triggered the workflow run, and is different for each event. The webhooks for each GitHub Actions event is linked in "Events that trigger workflows." For example, for a workflow run triggered by the push event, this object contains the contents of the push webhook payload.
		"event_name":        run.TriggerEvent,                         // string, The name of the event that triggered the workflow run.
		"event_path":        "",                                       // string, The path to the file on the runner that contains the full event webhook payload.
		"graphql_url":       "",                                       // string, The URL of the GitHub GraphQL API.
		"head_ref":          headRef,                                  // string, The head_ref or source branch of the pull request in a workflow run. This property is only available when the event that triggers a workflow run is either pull_request or pull_request_target.
		"job":               "",                                       // string, The job_id of the current job.
		"ref":               ref,                                      // string, The fully-formed ref of the branch or tag that triggered the workflow run. For workflows triggered by push, this is the branch or tag ref that was pushed. For workflows triggered by pull_request, this is the pull request merge branch. For workflows triggered by release, this is the release tag created. For other triggers, this is the branch or tag ref that triggered the workflow run. This is only set if a branch or tag is available for the event type. The ref given is fully-formed, meaning that for branches the format is refs/heads/<branch_name>, for pull requests it is refs/pull/<pr_number>/merge, and for tags it is refs/tags/<tag_name>. For example, refs/heads/feature-branch-1.
		"ref_name":          refName.ShortName(),                      // string, The short ref name of the branch or tag that triggered the workflow run. This value matches the branch or tag name shown on GitHub. For example, feature-branch-1.
		"ref_protected":     false,                                    // boolean, true if branch protections are configured for the ref that triggered the workflow run.
		"ref_type":          string(refName.RefType()),                // string, The type of ref that triggered the workflow run. Valid values are branch or tag.
		"path":              "",                                       // string, Path on the runner to the file that sets system PATH variables from workflow commands. This file is unique to the current step and is a different file for each step in a job. For more information, see "Workflow commands for GitHub Actions."
		"repository":        run.Repo.OwnerName + "/" + run.Repo.Name, // string, The owner and repository name. For example, Codertocat/Hello-World.
		"repository_owner":  run.Repo.OwnerName,                       // string, The repository owner's name. For example, Codertocat.
		"repositoryUrl":     run.Repo.HTMLURL(),                       // string, The Git URL to the repository. For example, git://github.com/codertocat/hello-world.git.
		"retention_days":    "",                                       // string, The number of days that workflow run logs and artifacts are kept.
		"run_id":            "",                                       // string, A unique number for each workflow run within a repository. This number does not change if you re-run the workflow run.
		"run_number":        fmt.Sprint(run.Index),                    // string, A unique number for each run of a particular workflow in a repository. This number begins at 1 for the workflow's first run, and increments with each new run. This number does not change if you re-run the workflow run.
		"run_attempt":       "",                                       // string, A unique number for each attempt of a particular workflow run in a repository. This number begins at 1 for the workflow run's first attempt, and increments with each re-run.
		"secret_source":     "Actions",                                // string, The source of a secret used in a workflow. Possible values are None, Actions, Dependabot, or Codespaces.
		"server_url":        setting.AppURL,                           // string, The URL of the GitHub server. For example: https://github.com.
		"sha":               sha,                                      // string, The commit SHA that triggered the workflow. The value of this commit SHA depends on the event that triggered the workflow. For more information, see "Events that trigger workflows." For example, ffac537e6cbbf934b08745a378932722df287a53.
		"triggering_actor":  "",                                       // string, The username of the user that initiated the workflow run. If the workflow run is a re-run, this value may differ from github.actor. Any workflow re-runs will use the privileges of github.actor, even if the actor initiating the re-run (github.triggering_actor) has different privileges.
		"workflow":          run.WorkflowID,                           // string, The name of the workflow. If the workflow file doesn't specify a name, the value of this property is the full path of the workflow file in the repository.
		"workspace":         "",                                       // string, The default working directory on the runner for steps, and the default location of your repository when using the checkout action.

		// additional contexts
		"gitea_default_actions_url": setting.Actions.DefaultActionsURL.URL(),
	}

	if job != nil {
		gitContext["job"] = job.JobID
		gitContext["run_id"] = fmt.Sprint(job.RunID)
		gitContext["run_attempt"] = fmt.Sprint(job.Attempt)
	}

	return gitContext
}

type TaskNeed struct {
	Result  actions_model.Status
	Outputs map[string]string
}

// FindTaskNeeds finds the `needs` for the task by the task's job
func FindTaskNeeds(ctx context.Context, job *actions_model.ActionRunJob) (map[string]*TaskNeed, error) {
	if len(job.Needs) == 0 {
		return nil, nil
	}
	needs := container.SetOf(job.Needs...)

	jobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: job.RunID})
	if err != nil {
		return nil, fmt.Errorf("FindRunJobs: %w", err)
	}

	jobIDJobs := make(map[string][]*actions_model.ActionRunJob)
	for _, job := range jobs {
		jobIDJobs[job.JobID] = append(jobIDJobs[job.JobID], job)
	}

	ret := make(map[string]*TaskNeed, len(needs))
	for jobID, jobsWithSameID := range jobIDJobs {
		if !needs.Contains(jobID) {
			continue
		}
		var jobOutputs map[string]string
		for _, job := range jobsWithSameID {
			if job.TaskID == 0 || !job.Status.IsDone() {
				// it shouldn't happen, or the job has been rerun
				continue
			}
			got, err := actions_model.FindTaskOutputByTaskID(ctx, job.TaskID)
			if err != nil {
				return nil, fmt.Errorf("FindTaskOutputByTaskID: %w", err)
			}
			outputs := make(map[string]string, len(got))
			for _, v := range got {
				outputs[v.OutputKey] = v.OutputValue
			}
			if len(jobOutputs) == 0 {
				jobOutputs = outputs
			} else {
				jobOutputs = mergeTwoOutputs(outputs, jobOutputs)
			}
		}
		ret[jobID] = &TaskNeed{
			Outputs: jobOutputs,
			Result:  actions_model.AggregateJobStatus(jobsWithSameID),
		}
	}
	return ret, nil
}

// mergeTwoOutputs merges two outputs from two different ActionRunJobs
// Values with the same output name may be overridden. The user should ensure the output names are unique.
// See https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions#using-job-outputs-in-a-matrix-job
func mergeTwoOutputs(o1, o2 map[string]string) map[string]string {
	ret := make(map[string]string, len(o1))
	for k1, v1 := range o1 {
		if len(v1) > 0 {
			ret[k1] = v1
		} else {
			ret[k1] = o2[k1]
		}
	}
	return ret
}
