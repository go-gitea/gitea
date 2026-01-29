// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
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
	act_pkg_runner "github.com/nektos/act/pkg/runner"
	"gopkg.in/yaml.v3"
	"xorm.io/builder"
)

func expandReusableWorkflow(ctx context.Context, parentJob *actions_model.ActionRunJob) error {
	workflowJob, err := parentJob.ParseJob()
	if err != nil {
		return err
	}
	ref, err := act_pkg_runner.ParseReusableWorkflowRef(workflowJob.Uses)
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

func createChildRunFromReusableWorkflow(ctx context.Context, parentJob *actions_model.ActionRunJob, workflowJob *jobparser.Job, ref *act_pkg_runner.ReusableWorkflowRef, vars map[string]string) error {
	if err := parentJob.LoadAttributes(ctx); err != nil {
		return err
	}
	parentJobRun := parentJob.Run

	if err := checkRunNestingLevel(ctx, parentJobRun); err != nil {
		return err
	}

	content, err := loadReusableWorkflowContent(ctx, parentJobRun, ref)
	if err != nil {
		return err
	}

	giteaCtx := GenerateGiteaContext(parentJob.Run, parentJob)

	inputsWithDefaults, err := buildWorkflowCallInputs(ctx, parentJob, workflowJob, content, vars, giteaCtx)
	if err != nil {
		return err
	}

	workflowCallPayload := &api.WorkflowCallPayload{
		Workflow:   parentJobRun.WorkflowID,
		Ref:        parentJobRun.Ref,
		Repository: convert.ToRepo(ctx, parentJobRun.Repo, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Sender:     convert.ToUserWithAccessMode(ctx, parentJobRun.TriggerUser, perm.AccessModeNone),
		Inputs:     inputsWithDefaults,
	}

	jobs, err := jobparser.Parse(content, jobparser.WithVars(vars), jobparser.WithGitContext(giteaCtx.ToGitHubContext()), jobparser.WithInputs(inputsWithDefaults))
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	var childRunName string
	if len(jobs) > 0 {
		if jobs[0].RunName != "" {
			childRunName = jobs[0].RunName
		} else if jobs[0].Name != "" {
			childRunName = jobs[0].Name
		}
	}
	if childRunName == "" {
		childRunName = path.Base(ref.WorkflowPath)
	}
	childRunTitle := fmt.Sprintf("%s / %s", parentJobRun.Title, parentJob.Name)

	var eventPayload []byte
	if eventPayload, err = workflowCallPayload.JSONPayload(); err != nil {
		return fmt.Errorf("JSONPayload: %w", err)
	}

	childRun := &actions_model.ActionRun{
		Title:       childRunTitle,
		RepoID:      parentJobRun.RepoID,
		OwnerID:     parentJobRun.OwnerID,
		ParentJobID: parentJob.ID,
		// A called workflow uses the name of its caller workflow in ${{ github.workflow }}
		// See https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations#supported-keywords-for-jobs-that-call-a-reusable-workflow
		WorkflowID:        parentJobRun.WorkflowID,
		TriggerUserID:     parentJobRun.TriggerUserID,
		TriggerUser:       parentJobRun.TriggerUser,
		Ref:               parentJobRun.Ref,
		CommitSHA:         parentJobRun.CommitSHA,
		IsForkPullRequest: parentJobRun.IsForkPullRequest,
		Event:             "workflow_call",
		TriggerEvent:      "workflow_call",
		EventPayload:      string(eventPayload),
		Status:            actions_model.StatusWaiting,
		NeedApproval:      parentJobRun.NeedApproval,
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

func buildWorkflowCallInputs(ctx context.Context, parentJob *actions_model.ActionRunJob, workflowJob *jobparser.Job, content []byte, vars map[string]string, giteaCtx GiteaContext) (map[string]any, error) {
	singleWorkflow := &jobparser.SingleWorkflow{}
	if err := yaml.Unmarshal(content, singleWorkflow); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	workflow := &actmodel.Workflow{
		RawOn: singleWorkflow.RawOn,
	}

	parentRunInputs, err := getInputsFromRun(parentJob.Run)
	if err != nil {
		return nil, fmt.Errorf("get parent run inputs: %w", err)
	}

	results, err := findJobNeedsAndFillJobResults(ctx, parentJob)
	if err != nil {
		return nil, fmt.Errorf("get job results: %w", err)
	}

	return jobparser.EvaluateWorkflowCallInputs(workflow, parentJob.JobID, workflowJob, giteaCtx, results, vars, parentRunInputs)
}

func loadReusableWorkflowContent(ctx context.Context, parentRun *actions_model.ActionRun, ref *act_pkg_runner.ReusableWorkflowRef) ([]byte, error) {
	if ref.Kind == act_pkg_runner.ReusableWorkflowKindLocalSameRepo {
		if err := parentRun.LoadRepo(ctx); err != nil {
			return nil, err
		}
		return readWorkflowContentFromRepo(ctx, parentRun.Repo, parentRun.Ref, ref.WorkflowPath)
	}

	if ref.Kind == act_pkg_runner.ReusableWorkflowKindLocalOtherRepo || isSameInstanceHost(ref.GitInstanceURL) {
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

	content, err := ref.FetchReusableWorkflowContent(ctx)
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

func isSameInstanceHost(usesInstanceURL string) bool {
	u1, err := url.Parse(setting.AppURL)
	if err != nil {
		return false
	}
	u2, err := url.Parse(usesInstanceURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u1.Host, u2.Host)
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

var ErrReusableWorkflowNestingLimitExceeded = errors.New("reusable workflow nesting limit exceeded")

// See: https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows#nesting-reusable-workflows
const maxNestingReusableWorkflowLevel = 9

// checkRunNestingLevel returns the number of parent runs from this run to the top-level run.
func checkRunNestingLevel(ctx context.Context, run *actions_model.ActionRun) error {
	depth := 0
	cur := run
	for cur.ParentJobID > 0 {
		if cur.ParentJobID == 0 {
			break
		}
		if err := cur.LoadParentJob(ctx); err != nil {
			return err
		}
		parentJob := cur.ParentJob
		if err := parentJob.LoadRun(ctx); err != nil {
			return err
		}
		cur = parentJob.Run
		depth++
		if depth >= maxNestingReusableWorkflowLevel {
			return ErrReusableWorkflowNestingLimitExceeded
		}
	}
	return nil
}
