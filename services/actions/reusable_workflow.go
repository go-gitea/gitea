// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/jobparser"
	actmodel "github.com/nektos/act/pkg/model"
	act_runner_pkg "github.com/nektos/act/pkg/runner"
	"gopkg.in/yaml.v3"
	"xorm.io/builder"
)

func expandReusableWorkflows(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob, vars map[string]string) error {
	for _, job := range jobs {
		if job.ChildRunID != -1 {
			// should not happen
			continue
		}
		workflowJob, err := job.ParseJob()
		if err != nil {
			return err
		}
		ref, err := act_runner_pkg.ParseReusableWorkflowRef(workflowJob.Uses)
		if err != nil {
			return err
		}
		if err := createChildRunFromReusableWorkflow(ctx, job, workflowJob, ref, vars); err != nil {
			return err
		}
	}
	return nil
}

func expandReusableWorkflow(ctx context.Context, parentJob *actions_model.ActionRunJob) error {
	if parentJob.ChildRunID != -1 {
		// should not happen
		return fmt.Errorf("no need to expand")
	}
	workflowJob, err := parentJob.ParseJob()
	if err != nil {
		return err
	}
	ref, err := act_runner_pkg.ParseReusableWorkflowRef(workflowJob.Uses)
	if err != nil {
		return err
	}
	if err := parentJob.LoadAttributes(ctx); err != nil {
		return err
	}
	vars, err := actions_model.GetVariablesOfRun(ctx, parentJob.Run)
	if err != nil {
		return err
	}
	if err := createChildRunFromReusableWorkflow(ctx, parentJob, workflowJob, ref, vars); err != nil {
		return err
	}
	return nil
}

func createChildRunFromReusableWorkflow(ctx context.Context, parentJob *actions_model.ActionRunJob, workflowJob *jobparser.Job, ref *act_runner_pkg.ReusableWorkflowRef, vars map[string]string) error {
	if err := parentJob.LoadAttributes(ctx); err != nil {
		return err
	}
	parentRun := parentJob.Run

	content, err := loadReusableWorkflowContent(ctx, parentRun, ref)
	if err != nil {
		return err
	}

	inputsWithDefaults, err := buildWorkflowCallInputs(ctx, parentJob, workflowJob, content, vars)
	if err != nil {
		return err
	}

	workflowCallPayload := &api.WorkflowCallPayload{
		Workflow:   parentRun.WorkflowID,
		Ref:        parentRun.Ref,
		Repository: convert.ToRepo(ctx, parentRun.Repo, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Sender:     convert.ToUserWithAccessMode(ctx, parentRun.TriggerUser, perm.AccessModeNone),
		Inputs:     inputsWithDefaults,
	}

	giteaCtx := GenerateGiteaContext(parentJob.Run, parentJob)

	jobs, err := jobparser.Parse(content, jobparser.WithVars(vars), jobparser.WithGitContext(giteaCtx.ToGitHubContext()), jobparser.WithInputs(inputsWithDefaults))
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	childRunName := ref.WorkflowPath
	if len(jobs) > 0 && jobs[0].RunName != "" {
		childRunName = jobs[0].RunName
	}

	var eventPayload []byte
	if eventPayload, err = workflowCallPayload.JSONPayload(); err != nil {
		return fmt.Errorf("JSONPayload: %w", err)
	}

	childRun := &actions_model.ActionRun{
		Title:       fmt.Sprintf("%s / %s", parentRun.Title, childRunName),
		RepoID:      parentRun.RepoID,
		OwnerID:     parentRun.OwnerID,
		ParentJobID: parentJob.ID,
		// A called workflow uses the name of its caller workflow in ${{ github.workflow }}
		// See https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations#supported-keywords-for-jobs-that-call-a-reusable-workflow
		WorkflowID:        parentRun.WorkflowID,
		TriggerUserID:     parentRun.TriggerUserID,
		TriggerUser:       parentRun.TriggerUser,
		Ref:               parentRun.Ref,
		CommitSHA:         parentRun.CommitSHA,
		IsForkPullRequest: parentRun.IsForkPullRequest,
		Event:             "workflow_call",
		TriggerEvent:      "workflow_call",
		EventPayload:      string(eventPayload),
		Status:            actions_model.StatusWaiting,
		NeedApproval:      parentRun.NeedApproval,
	}

	if err := PrepareRunAndInsert(ctx, content, childRun, inputsWithDefaults); err != nil {
		return err
	}
	parentJob.ChildRunID = childRun.ID
	if _, err := actions_model.UpdateRunJob(ctx, parentJob, builder.Eq{"child_run_id": -1}, "child_run_id"); err != nil {
		return err
	}
	return nil
}

func buildWorkflowCallInputs(ctx context.Context, parentJob *actions_model.ActionRunJob, workflowJob *jobparser.Job, content []byte, vars map[string]string) (map[string]any, error) {
	singleWorkflow := &jobparser.SingleWorkflow{}
	if err := yaml.Unmarshal(content, singleWorkflow); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	workflow := &actmodel.Workflow{
		RawOn: singleWorkflow.RawOn,
	}

	giteaCtx := GenerateGiteaContext(parentJob.Run, parentJob)

	inputs, err := getInputsFromRun(parentJob.Run)
	if err != nil {
		return nil, fmt.Errorf("get inputs: %w", err)
	}

	results, err := findJobNeedsAndFillJobResults(ctx, parentJob)
	if err != nil {
		return nil, fmt.Errorf("get job results: %w", err)
	}

	return jobparser.EvaluateWorkflowCallInputs(workflow, parentJob.JobID, workflowJob, giteaCtx, results, vars, inputs)
}

func loadReusableWorkflowContent(ctx context.Context, parentRun *actions_model.ActionRun, ref *act_runner_pkg.ReusableWorkflowRef) ([]byte, error) {
	if ref.Kind == act_runner_pkg.ReusableWorkflowKindLocalSameRepo {
		if err := parentRun.LoadRepo(ctx); err != nil {
			return nil, err
		}
		return readWorkflowContentFromRepo(ctx, parentRun.Repo, parentRun.Ref, ref.WorkflowPath)
	}

	if ref.Kind == act_runner_pkg.ReusableWorkflowKindLocalOtherRepo || isSameInstanceHost(ref.Host) {
		repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ref.Owner, ref.Repo)
		if err != nil {
			return nil, err
		}
		if repo.IsPrivate {
			perm, err := access_model.GetActionsUserRepoPermissionByActionRun(ctx, repo, user_model.NewActionsUser(), parentRun)
			if err != nil {
				return nil, err
			}
			if !perm.CanRead(unit.TypeCode) {
				return nil, errors.New("actions user has no access to reusable workflow repo")
			}
		}
		return readWorkflowContentFromRepo(ctx, repo, ref.Ref, ref.WorkflowPath)
	}

	content, err := act_runner_pkg.FetchReusableWorkflowContent(ctx, ref)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func readWorkflowContentFromRepo(ctx context.Context, repo *repo_model.Repository, ref, workflowPath string) ([]byte, error) {
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}
	content, err := commit.GetFileContent(workflowPath, 1024*1024)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

func isSameInstanceHost(host string) bool {
	appURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return false
	}
	h, err := url.Parse(host)
	if err != nil {
		return false
	}
	return strings.EqualFold(h.Host, appURL.Host)
}

func markChildRunJobsSkipped(ctx context.Context, childRunJobs []*actions_model.ActionRunJob) error {
	for _, job := range childRunJobs {
		oldStatus := job.Status
		if !oldStatus.IsBlocked() {
			continue
		}
		job.Status = actions_model.StatusSkipped
		if _, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": oldStatus}, "status"); err != nil {
			return err
		}

		if job.ChildRunID > 0 {
			jobs, err := actions_model.GetRunJobsByRunID(ctx, job.ChildRunID)
			if err != nil {
				return err
			}
			if err := markChildRunJobsSkipped(ctx, jobs); err != nil {
				return err
			}
		}
	}
	return nil
}
