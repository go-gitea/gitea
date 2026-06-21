// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strconv"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"

	"gitea.com/gitea/runner/act/model"
)

type GiteaContext map[string]any

// GenerateGiteaContext generate the gitea context without token and gitea_runtime_token.
// attempt and job can be nil when generating a context for parsing workflow-level expressions.
//
// The run_attempt value is resolved with the following precedence:
//  1. attempt.Attempt - the explicit attempt argument, or run.GetLatestAttempt() as a fallback
//  2. job.Attempt - only used when neither an explicit nor latest attempt is available
//  3. "1" - when none of the above apply (first-run parse time, before the first attempt exists)
func GenerateGiteaContext(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, job *actions_model.ActionRunJob) GiteaContext {
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

	gitContext := GiteaContext{
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
		"run_id":            strconv.FormatInt(run.ID, 10),            // string, A unique number for each workflow run within a repository. This number does not change if you re-run the workflow run.
		"run_number":        strconv.FormatInt(run.Index, 10),         // string, A unique number for each run of a particular workflow in a repository. This number begins at 1 for the workflow's first run, and increments with each new run. This number does not change if you re-run the workflow run.
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
		gitContext["run_attempt"] = strconv.FormatInt(job.Attempt, 10)

		if job.ParentJobID > 0 {
			// Inject the caller's resolved workflow_call inputs into gitea.event.inputs.
			// The rest of gitea.event stays as the caller's actual trigger event (push/pull_request/etc.)
			// to match GitHub's semantics (see https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations#github-context).
			// FIXME: If the run is triggered by "workflow_dispatch", the original inputs of "workflow_dispatch" will be overridden.
			// If necessary, the caller can send these values to the called workflow via `with:`.
			caller, err := actions_model.GetRunJobByRunAndID(ctx, job.RunID, job.ParentJobID)
			if err != nil {
				log.Error("GenerateGiteaContext: load caller job %d of job %d: %v", job.ParentJobID, job.ID, err)
			} else if caller.CallPayload != "" {
				var cp api.WorkflowCallPayload
				if err := json.Unmarshal([]byte(caller.CallPayload), &cp); err != nil {
					log.Error("GenerateGiteaContext: decode CallPayload of caller %d: %v", caller.ID, err)
				} else if cp.Inputs != nil {
					event["inputs"] = cp.Inputs
				}
			}

			// Override gitea.event_name to "workflow_call", so that the runner-side `getEvaluatorInputs` can get inputs from event["inputs"].
			// https://gitea.com/gitea/runner/src/commit/0b9f251b6abb30d5f292a49cfe0c611f7c26d857/act/runner/expression.go#L509
			// FIXME: The trade-off is that `${{ gitea.event_name }}` inside a reusable workflow's child job reads "workflow_call"
			// instead of the caller's real trigger event name (push/pull_request/etc.) This is a small deviation from GitHub spec.
			gitContext["event_name"] = "workflow_call"
		}
	}

	if attempt == nil {
		if latestAttempt, has, err := run.GetLatestAttempt(ctx); err == nil && has {
			attempt = latestAttempt
		}
	}

	if attempt != nil {
		gitContext["run_attempt"] = strconv.FormatInt(attempt.Attempt, 10)
		if err := attempt.LoadAttributes(ctx); err == nil {
			gitContext["triggering_actor"] = attempt.TriggerUser.Name
		}
	}

	// Fallback for first-run parse time: no job, no attempt (LatestAttemptID==0). github.run_attempt
	// is 1-based per the documented contract, so emit "1" rather than leaving it empty.
	if gitContext["run_attempt"] == "" {
		gitContext["run_attempt"] = "1"
	}

	return gitContext
}

type TaskNeed struct {
	Result  actions_model.Status
	Outputs map[string]string
}

// FindTaskNeeds finds the `needs` for the task by the task's job.
// Lookup is scoped to the same ParentJobID.
func FindTaskNeeds(ctx context.Context, job *actions_model.ActionRunJob) (map[string]*TaskNeed, error) {
	if len(job.Needs) == 0 {
		return nil, nil //nolint:nilnil // return nil when the job has no needs
	}
	needs := container.SetOf(job.Needs...)

	// Scope to the same attempt. For legacy jobs RunAttemptID==0, which matches all other legacy jobs in the same run.
	findOpts := actions_model.FindRunJobOptions{
		RunID:        job.RunID,
		RunAttemptID: optional.Some(job.RunAttemptID),
	}

	jobs, err := db.Find[actions_model.ActionRunJob](ctx, findOpts)
	if err != nil {
		return nil, fmt.Errorf("FindRunJobs: %w", err)
	}

	jobIDJobs := make(map[string][]*actions_model.ActionRunJob)
	// childrenByParent indexes every job by its ParentJobID
	childrenByParent := make(map[int64][]*actions_model.ActionRunJob)
	for _, candidate := range jobs {
		if candidate.ParentJobID != 0 {
			childrenByParent[candidate.ParentJobID] = append(childrenByParent[candidate.ParentJobID], candidate)
		}
		// `needs` references are scope-bound: only candidates in the same caller scope match.
		if candidate.ParentJobID == job.ParentJobID {
			jobIDJobs[candidate.JobID] = append(jobIDJobs[candidate.JobID], candidate)
		}
	}

	ret := make(map[string]*TaskNeed, len(needs))
	for jobID, jobsWithSameID := range jobIDJobs {
		if !needs.Contains(jobID) {
			continue
		}
		var jobOutputs map[string]string
		for _, candidate := range jobsWithSameID {
			if !candidate.Status.IsDone() {
				continue
			}
			var outputs map[string]string
			var err error
			if candidate.IsReusableCaller {
				outputs, err = computeReusableCallerOutputs(ctx, candidate, childrenByParent)
			} else {
				outputs, err = loadJobTaskOutputs(ctx, candidate)
			}
			if err != nil {
				return nil, err
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

// computeReusableCallerOutputs returns the workflow_call outputs of a reusable caller by recursing into its child subtree.
func computeReusableCallerOutputs(ctx context.Context, caller *actions_model.ActionRunJob, childrenByParent map[int64][]*actions_model.ActionRunJob) (map[string]string, error) {
	if !caller.IsExpanded {
		//  A caller that was never expanded (e.g. Skipped because its `if:` was false) has no workflow_call outputs, return early.
		return map[string]string{}, nil
	}

	directChildren := childrenByParent[caller.ID]

	if err := caller.LoadRun(ctx); err != nil {
		return nil, err
	}
	wcSpec, err := jobparser.ParseWorkflowCallSpec(caller.ReusableWorkflowContent)
	if err != nil {
		return nil, err
	}
	if len(wcSpec.Outputs) == 0 {
		return map[string]string{}, nil
	}

	// Per-job outputs over the children of this caller.
	jobOutputs := make(jobparser.JobOutputs, len(directChildren))
	for _, child := range directChildren {
		var outs map[string]string
		switch {
		case child.IsReusableCaller:
			outs, err = computeReusableCallerOutputs(ctx, child, childrenByParent)
		default:
			outs, err = loadJobTaskOutputs(ctx, child)
		}
		if err != nil {
			return nil, err
		}
		if existing, ok := jobOutputs[child.JobID]; ok {
			jobOutputs[child.JobID] = mergeTwoOutputs(outs, existing)
		} else {
			jobOutputs[child.JobID] = outs
		}
	}

	// build contexts for evaluating outputs
	if err := caller.Run.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	gitCtx := GenerateGiteaContext(ctx, caller.Run, nil, caller)
	vars, err := actions_model.GetVariablesOfRun(ctx, caller.Run)
	if err != nil {
		return nil, err
	}
	inputs := map[string]any{}
	if caller.CallPayload != "" {
		var p api.WorkflowCallPayload
		if err := json.Unmarshal([]byte(caller.CallPayload), &p); err != nil {
			return nil, fmt.Errorf("decode caller payload: %w", err)
		}
		if p.Inputs != nil {
			inputs = p.Inputs
		}
	}

	return jobparser.EvaluateWorkflowCallOutputs(wcSpec, gitCtx.ToGitHubContext(), vars, inputs, jobOutputs)
}

// loadJobTaskOutputs returns the task-output map of `job`.
func loadJobTaskOutputs(ctx context.Context, job *actions_model.ActionRunJob) (map[string]string, error) {
	tid := job.EffectiveTaskID()
	if tid == 0 {
		return map[string]string{}, nil
	}
	rows, err := actions_model.FindTaskOutputByTaskID(ctx, tid)
	if err != nil {
		return nil, fmt.Errorf("FindTaskOutputByTaskID: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.OutputKey] = r.OutputValue
	}
	return out, nil
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

func (g *GiteaContext) ToGitHubContext() *model.GithubContext {
	return &model.GithubContext{
		Event:            util.GetMapValueOrDefault(*g, "event", map[string]any(nil)),
		EventPath:        util.GetMapValueOrDefault(*g, "event_path", ""),
		Workflow:         util.GetMapValueOrDefault(*g, "workflow", ""),
		RunID:            util.GetMapValueOrDefault(*g, "run_id", ""),
		RunNumber:        util.GetMapValueOrDefault(*g, "run_number", ""),
		Actor:            util.GetMapValueOrDefault(*g, "actor", ""),
		Repository:       util.GetMapValueOrDefault(*g, "repository", ""),
		EventName:        util.GetMapValueOrDefault(*g, "event_name", ""),
		Sha:              util.GetMapValueOrDefault(*g, "sha", ""),
		Ref:              util.GetMapValueOrDefault(*g, "ref", ""),
		RefName:          util.GetMapValueOrDefault(*g, "ref_name", ""),
		RefType:          util.GetMapValueOrDefault(*g, "ref_type", ""),
		HeadRef:          util.GetMapValueOrDefault(*g, "head_ref", ""),
		BaseRef:          util.GetMapValueOrDefault(*g, "base_ref", ""),
		Token:            "", // deliberately omitted for security
		Workspace:        util.GetMapValueOrDefault(*g, "workspace", ""),
		Action:           util.GetMapValueOrDefault(*g, "action", ""),
		ActionPath:       util.GetMapValueOrDefault(*g, "action_path", ""),
		ActionRef:        util.GetMapValueOrDefault(*g, "action_ref", ""),
		ActionRepository: util.GetMapValueOrDefault(*g, "action_repository", ""),
		Job:              util.GetMapValueOrDefault(*g, "job", ""),
		JobName:          "", // not present in GiteaContext
		RepositoryOwner:  util.GetMapValueOrDefault(*g, "repository_owner", ""),
		RetentionDays:    util.GetMapValueOrDefault(*g, "retention_days", ""),
		RunnerPerflog:    "", // not present in GiteaContext
		RunnerTrackingID: "", // not present in GiteaContext
		ServerURL:        util.GetMapValueOrDefault(*g, "server_url", ""),
		APIURL:           util.GetMapValueOrDefault(*g, "api_url", ""),
		GraphQLURL:       util.GetMapValueOrDefault(*g, "graphql_url", ""),
	}
}
