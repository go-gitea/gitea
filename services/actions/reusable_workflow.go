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
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"

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
		if err := createChildRunFromReusableWorkflow(ctx, run, job, workflowJob, ref, vars); err != nil {
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
	if err := createChildRunFromReusableWorkflow(ctx, parentJob.Run, parentJob, workflowJob, ref, vars); err != nil {
		return err
	}
	return nil
}

func createChildRunFromReusableWorkflow(ctx context.Context, run *actions_model.ActionRun, parentJob *actions_model.ActionRunJob, workflowJob *jobparser.Job, ref *act_runner_pkg.ReusableWorkflowRef, vars map[string]string) error {
	content, err := loadReusableWorkflowContent(ctx, run, ref)
	if err != nil {
		return err
	}

	inputsWithDefaults, err := buildWorkflowCallInputs(ctx, run, parentJob, workflowJob, content, vars)
	if err != nil {
		return err
	}

	eventPayload, err := json.Marshal(map[string]any{
		"inputs": inputsWithDefaults,
	})
	if err != nil {
		return err
	}

	childRun := &actions_model.ActionRun{
		Title:             run.Title,
		RepoID:            run.RepoID,
		Repo:              run.Repo,
		OwnerID:           run.OwnerID,
		ParentJobID:       parentJob.ID,
		WorkflowID:        ref.WorkflowPath,
		TriggerUserID:     run.TriggerUserID,
		TriggerUser:       run.TriggerUser,
		Ref:               run.Ref,
		CommitSHA:         run.CommitSHA,
		IsForkPullRequest: run.IsForkPullRequest,
		Event:             "workflow_call",
		TriggerEvent:      "workflow_call",
		EventPayload:      string(eventPayload),
		Status:            actions_model.StatusWaiting,
		NeedApproval:      run.NeedApproval,
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

func buildWorkflowCallInputs(ctx context.Context, run *actions_model.ActionRun, parentJob *actions_model.ActionRunJob, workflowJob *jobparser.Job, content []byte, vars map[string]string) (map[string]any, error) {
	singleWorkflow := &jobparser.SingleWorkflow{}
	if err := yaml.Unmarshal(content, singleWorkflow); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	workflow := &actmodel.Workflow{
		RawOn: singleWorkflow.RawOn,
	}

	giteaCtx := GenerateGiteaContext(run, parentJob)
	inputs, err := getInputsFromRun(run)
	if err != nil {
		return nil, fmt.Errorf("get inputs: %w", err)
	}

	results, err := findJobNeedsAndFillJobResults(ctx, parentJob)
	if err != nil {
		return nil, fmt.Errorf("get job results: %w", err)
	}

	return jobparser.EvaluateWorkflowCallInputs(workflow, parentJob.JobID, workflowJob, giteaCtx, results, vars, inputs)
}

func loadReusableWorkflowContent(ctx context.Context, run *actions_model.ActionRun, ref *act_runner_pkg.ReusableWorkflowRef) ([]byte, error) {
	if ref.Kind == act_runner_pkg.ReusableWorkflowKindLocal {
		if err := run.LoadRepo(ctx); err != nil {
			return nil, err
		}
		return readWorkflowContentFromRepo(ctx, run.Repo, run.Ref, ref.WorkflowPath)
	}

	if ref.Kind == act_runner_pkg.ReusableWorkflowKindOtherRepo || isSameInstanceHost(ref.Host) {
		repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ref.Owner, ref.Repo)
		if err != nil {
			return nil, err
		}
		if repo.IsPrivate {
			perm, err := access_model.GetActionsUserRepoPermissionByRepoID(ctx, repo, user_model.NewActionsUser(), run.RepoID, run.IsForkPullRequest)
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
