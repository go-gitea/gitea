// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	actions_service "code.gitea.io/gitea/services/actions"
	context_module "code.gitea.io/gitea/services/context"

	"gitea.com/gitea/runner/act/model"
)

func findCurrentJobByPathParam(ctx *context_module.Context, jobs []*actions_model.ActionRunJob) (job *actions_model.ActionRunJob, hasPathParam bool) {
	selectedJobID := ctx.PathParamInt64("job")
	if selectedJobID <= 0 {
		return nil, false
	}
	for _, job = range jobs {
		if job.ID == selectedJobID {
			return job, true
		}
	}
	return nil, true
}

func getCurrentRunByPathParam(ctx *context_module.Context) (run *actions_model.ActionRun) {
	var err error
	// if run param is "latest", get the latest run id
	if ctx.PathParam("run") == "latest" {
		run, err = actions_model.GetLatestRun(ctx, ctx.Repo.Repository.ID)
	} else {
		run, err = actions_model.GetRunByRepoAndID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("run"))
	}
	if errors.Is(err, util.ErrNotExist) {
		ctx.NotFound(nil)
	} else if err != nil {
		ctx.ServerError("GetRun:"+ctx.PathParam("run"), err)
	}
	return run
}

// resolveCurrentRunForView resolves GET Actions page URLs and supports both ID-based and legacy index-based forms.
//
// By default, run summary pages (/actions/runs/{run}) use a best-effort ID-first fallback,
// and job pages (/actions/runs/{run}/jobs/{job}) try to confirm an ID-based URL first and prefer the ID-based interpretation when both are valid.
//
// `by_id=1` param explicitly forces the ID-based path, and `by_index=1` explicitly forces the legacy index-based path.
// If both are present, `by_id` takes precedence.
func resolveCurrentRunForView(ctx *context_module.Context) *actions_model.ActionRun {
	// `by_id` explicitly requests ID-based resolution, so the request skips the legacy index-based disambiguation logic and resolves the run by ID directly.
	// It takes precedence over `by_index` when both query parameters are present.
	if ctx.PathParam("run") == "latest" || ctx.FormBool("by_id") {
		return getCurrentRunByPathParam(ctx)
	}

	runNum := ctx.PathParamInt64("run")
	if runNum <= 0 {
		ctx.NotFound(nil)
		return nil
	}

	byIndex := ctx.FormBool("by_index")

	if ctx.PathParam("job") == "" {
		// The URL does not contain a {job} path parameter, so it cannot use the
		// job-specific rules to disambiguate ID-based URLs from legacy index-based URLs.
		// Because of that, this path is handled with a best-effort ID-first fallback by default.
		//
		// When the same repository contains:
		//  - a run whose ID matches runNum, and
		//  - a different run whose repo-scope index also matches runNum
		// this path prefers the ID match and may show a different run than the old legacy URL originally intended,
		// unless `by_index=1` explicitly forces the legacy index-based interpretation.

		if !byIndex {
			runByID, err := actions_model.GetRunByRepoAndID(ctx, ctx.Repo.Repository.ID, runNum)
			if err == nil {
				return runByID
			}
			if !errors.Is(err, util.ErrNotExist) {
				ctx.ServerError("GetRun:"+ctx.PathParam("run"), err)
				return nil
			}
		}

		runByIndex, err := actions_model.GetRunByRepoAndIndex(ctx, ctx.Repo.Repository.ID, runNum)
		if err == nil {
			ctx.Redirect(fmt.Sprintf("%s/actions/runs/%d", ctx.Repo.RepoLink, runByIndex.ID), http.StatusFound)
			return nil
		}
		if !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetRunByRepoAndIndex", err)
			return nil
		}
		ctx.NotFound(nil)
		return nil
	}

	jobNum := ctx.PathParamInt64("job")
	if jobNum < 0 {
		ctx.NotFound(nil)
		return nil
	}

	// A job index should not be larger than MaxJobNumPerRun, so larger values can skip the legacy index-based path and be treated as job IDs directly.
	if !byIndex && jobNum >= actions_model.MaxJobNumPerRun {
		return getCurrentRunByPathParam(ctx)
	}

	var runByID, runByIndex *actions_model.ActionRun
	var targetJobByIndex *actions_model.ActionRunJob

	// Each run must have at least one job, so a valid job ID in the same run cannot be smaller than the run ID.
	if !byIndex && jobNum >= runNum {
		// Probe the repo-scoped job ID first and only accept it when the job exists and belongs to the same runNum.
		job, err := actions_model.GetRunJobByRepoAndID(ctx, ctx.Repo.Repository.ID, jobNum)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetRunJobByRepoAndID", err)
			return nil
		}
		if job != nil {
			if err := job.LoadRun(ctx); err != nil {
				ctx.ServerError("LoadRun", err)
				return nil
			}
			if job.Run.ID == runNum {
				runByID = job.Run
			}
		}
	}

	// Try to resolve the request as a legacy run-index/job-index URL.
	{
		run, err := actions_model.GetRunByRepoAndIndex(ctx, ctx.Repo.Repository.ID, runNum)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetRunByRepoAndIndex", err)
			return nil
		}
		if run != nil {
			jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, run.RepoID, run.ID)
			if err != nil {
				ctx.ServerError("GetRunJobsByRunID", err)
				return nil
			}
			if jobNum < int64(len(jobs)) {
				runByIndex = run
				targetJobByIndex = jobs[jobNum]
			}
		}
	}

	if runByID == nil && runByIndex == nil {
		ctx.NotFound(nil)
		return nil
	}

	if runByID != nil && runByIndex == nil {
		return runByID
	}

	if runByID == nil && runByIndex != nil {
		ctx.Redirect(fmt.Sprintf("%s/actions/runs/%d/jobs/%d", ctx.Repo.RepoLink, runByIndex.ID, targetJobByIndex.ID), http.StatusFound)
		return nil
	}

	// Reaching this point means both ID-based and legacy index-based interpretations are valid. Prefer the ID-based interpretation by default.
	// Use `by_index=1` query parameter to access the legacy index-based interpretation when necessary.
	return runByID
}

func View(ctx *context_module.Context) {
	ctx.Data["PageIsActions"] = true
	run := resolveCurrentRunForView(ctx)
	if ctx.Written() {
		return
	}
	run.Repo = ctx.Repo.Repository

	jobID := ctx.PathParamInt64("job")
	ctx.Data["JobID"] = jobID // it can be 0 when no job (e.g.: run summary view)

	attemptNum := ctx.PathParamInt64("attempt")

	// ActionsViewURL is the endpoint for viewing a run (job summary), a job, or a job attempt.
	// It's POST method handler can provide the state data for the frontend rendering.
	switch {
	case attemptNum > 0:
		ctx.Data["ActionsViewURL"] = fmt.Sprintf("%s/attempts/%d", run.Link(), attemptNum)
	case jobID > 0:
		ctx.Data["ActionsViewURL"] = fmt.Sprintf("%s/jobs/%d", run.Link(), jobID)
	default:
		ctx.Data["ActionsViewURL"] = run.Link()
	}

	ctx.HTML(http.StatusOK, tplViewActions)
}

func ViewWorkflowFile(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(run.CommitSHA)
	if err != nil {
		ctx.NotFoundOrServerError("GetCommit", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}
	rpath, entries, err := actions.ListWorkflows(commit)
	if err != nil {
		ctx.ServerError("ListWorkflows", err)
		return
	}
	for _, entry := range entries {
		if entry.Name() == run.WorkflowID {
			ctx.Redirect(fmt.Sprintf("%s/src/commit/%s/%s/%s", ctx.Repo.RepoLink, url.PathEscape(run.CommitSHA), util.PathEscapeSegments(rpath), util.PathEscapeSegments(run.WorkflowID)))
			return
		}
	}
	ctx.NotFound(nil)
}

type LogCursor struct {
	Step     int   `json:"step"`
	Cursor   int64 `json:"cursor"`
	Expanded bool  `json:"expanded"`
}

type ViewRequest struct {
	LogCursors []LogCursor `json:"logCursors"`
}

type ArtifactsViewItem struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	ExpiresUnix int64  `json:"expiresUnix"`
}

type ViewResponse struct {
	Artifacts []*ArtifactsViewItem `json:"artifacts"`

	State struct {
		Run struct {
			RepoID int64 `json:"repoId"`
			// Link is the canonical HTML URL of the run, e.g. "/owner/repo/actions/runs/123".
			// Used as the base for composing sub-resource URLs (cancel, rerun, artifacts, jobs) that are not attempt-scoped.
			Link string `json:"link"`
			// ViewLink is the attempt-aware URL for navigation, e.g. "/owner/repo/actions/runs/123" for the latest attempt
			// or "/owner/repo/actions/runs/123/attempts/2" for a historical attempt.
			// Use this when the target should reflect the currently-viewed attempt.
			ViewLink          string            `json:"viewLink"`
			Title             string            `json:"title"`
			TitleHTML         template.HTML     `json:"titleHTML"`
			Status            string            `json:"status"`
			CanCancel         bool              `json:"canCancel"`
			CanApprove        bool              `json:"canApprove"` // the run needs an approval and the doer has permission to approve
			CanRerun          bool              `json:"canRerun"`
			CanRerunFailed    bool              `json:"canRerunFailed"`
			CanDeleteArtifact bool              `json:"canDeleteArtifact"`
			Done              bool              `json:"done"`
			WorkflowID        string            `json:"workflowID"`
			WorkflowLink      string            `json:"workflowLink"`
			IsSchedule        bool              `json:"isSchedule"`
			RunAttempt        int64             `json:"runAttempt"`
			Attempts          []*ViewRunAttempt `json:"attempts"`
			Jobs              []*ViewJob        `json:"jobs"`
			Commit            ViewCommit        `json:"commit"`
			// Summary view: run duration and trigger time/event
			Duration     string `json:"duration"`
			TriggeredAt  int64  `json:"triggeredAt"`  // unix seconds for relative time
			TriggerEvent string `json:"triggerEvent"` // e.g. pull_request, push, schedule
		} `json:"run"`
		CurrentJob struct {
			Title  string         `json:"title"`
			Detail string         `json:"detail"`
			Steps  []*ViewJobStep `json:"steps"`
		} `json:"currentJob"`
	} `json:"state"`
	Logs struct {
		StepsLog []*ViewStepLog `json:"stepsLog"`
	} `json:"logs"`
}

type ViewJob struct {
	ID       int64    `json:"id"`
	Link     string   `json:"link"`
	JobID    string   `json:"jobId,omitempty"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	CanRerun bool     `json:"canRerun"`
	Duration string   `json:"duration"`
	Needs    []string `json:"needs,omitempty"`
}

type ViewRunAttempt struct {
	Attempt         int64  `json:"attempt"`
	Status          string `json:"status"`
	Done            bool   `json:"done"`
	Link            string `json:"link"`
	Current         bool   `json:"current"`
	Latest          bool   `json:"latest"`
	TriggeredAt     int64  `json:"triggeredAt"`
	TriggerUserName string `json:"triggerUserName"`
	TriggerUserLink string `json:"triggerUserLink"`
}

type ViewCommit struct {
	ShortSha string     `json:"shortSHA"`
	Link     string     `json:"link"`
	Pusher   ViewUser   `json:"pusher"`
	Branch   ViewBranch `json:"branch"`
}

type ViewUser struct {
	DisplayName string `json:"displayName"`
	Link        string `json:"link"`
}

type ViewBranch struct {
	Name      string `json:"name"`
	Link      string `json:"link"`
	IsDeleted bool   `json:"isDeleted"`
}

type ViewJobStep struct {
	Summary  string `json:"summary"`
	Duration string `json:"duration"`
	Status   string `json:"status"`
}

type ViewStepLog struct {
	Step    int                `json:"step"`
	Cursor  int64              `json:"cursor"`
	Lines   []*ViewStepLogLine `json:"lines"`
	Started int64              `json:"started"`
}

type ViewStepLogLine struct {
	Index     int64   `json:"index"`
	Message   string  `json:"message"`
	Timestamp float64 `json:"timestamp"`
}

func ViewPost(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if err := run.LoadAttributes(ctx); err != nil {
		ctx.ServerError("run.LoadAttributes", err)
		return
	}

	resp := &ViewResponse{}
	fillViewRunResponseSummary(ctx, resp, run, attempt, jobs)
	if ctx.Written() {
		return
	}
	fillViewRunResponseCurrentJob(ctx, resp, run, jobs)
	if ctx.Written() {
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

func fillViewRunResponseSummary(ctx *context_module.Context, resp *ViewResponse, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, jobs []*actions_model.ActionRunJob) {
	// Latest when the run has no attempts yet (legacy) or the viewed attempt is the run's latest.
	isLatestAttempt := run.LatestAttemptID == 0 || (attempt != nil && attempt.ID == run.LatestAttemptID)

	resp.State.Run.RepoID = ctx.Repo.Repository.ID
	// the title for the "run" is from the commit message
	resp.State.Run.Title = run.Title
	resp.State.Run.TitleHTML = templates.NewRenderUtils(ctx).RenderCommitMessage(run.Title, ctx.Repo.Repository)
	resp.State.Run.Link = run.Link()
	resp.State.Run.ViewLink = getRunViewLink(run, attempt)
	resp.State.Run.Attempts = make([]*ViewRunAttempt, 0)
	if attempt != nil {
		resp.State.Run.RunAttempt = attempt.Attempt
		resp.State.Run.Status = attempt.Status.String()
		resp.State.Run.Done = attempt.Status.IsDone()
		resp.State.Run.Duration = attempt.Duration().String()
		resp.State.Run.TriggeredAt = attempt.Created.AsTime().Unix()
	} else {
		resp.State.Run.Status = run.Status.String()
		resp.State.Run.Done = run.Status.IsDone()
		resp.State.Run.Duration = run.Duration().String()
		resp.State.Run.TriggeredAt = run.Created.AsTime().Unix()
	}
	resp.State.Run.CanCancel = isLatestAttempt && !resp.State.Run.Done && ctx.Repo.Permission.CanWrite(unit.TypeActions)
	resp.State.Run.CanApprove = isLatestAttempt && run.NeedApproval && ctx.Repo.Permission.CanWrite(unit.TypeActions)
	resp.State.Run.CanRerun = isLatestAttempt && resp.State.Run.Done && ctx.Repo.Permission.CanWrite(unit.TypeActions)
	resp.State.Run.CanDeleteArtifact = resp.State.Run.Done && ctx.Repo.Permission.CanWrite(unit.TypeActions)
	if resp.State.Run.CanRerun {
		for _, job := range jobs {
			if job.Status == actions_model.StatusFailure || job.Status == actions_model.StatusCancelled {
				resp.State.Run.CanRerunFailed = true
				break
			}
		}
	}
	resp.State.Run.WorkflowID = run.WorkflowID
	if isLatestAttempt {
		resp.State.Run.WorkflowLink = run.WorkflowLink()
	}
	resp.State.Run.IsSchedule = run.IsSchedule()
	resp.State.Run.Jobs = make([]*ViewJob, 0, len(jobs)) // marshal to '[]' instead fo 'null' in json
	for _, v := range jobs {
		resp.State.Run.Jobs = append(resp.State.Run.Jobs, &ViewJob{
			ID:       v.ID,
			Link:     fmt.Sprintf("%s/jobs/%d", run.Link(), v.ID),
			JobID:    v.JobID,
			Name:     v.Name,
			Status:   v.Status.String(),
			CanRerun: resp.State.Run.CanRerun,
			Duration: v.Duration().String(),
			Needs:    v.Needs,
		})
	}

	attempts, err := actions_model.ListRunAttemptsByRunID(ctx, run.ID)
	if err != nil {
		ctx.ServerError("ListRunAttemptsByRunID", err)
		return
	}
	if err := attempts.LoadTriggerUser(ctx); err != nil {
		ctx.ServerError("LoadTriggerUser", err)
		return
	}
	for _, runAttempt := range attempts {
		resp.State.Run.Attempts = append(resp.State.Run.Attempts, &ViewRunAttempt{
			Attempt:         runAttempt.Attempt,
			Status:          runAttempt.Status.String(),
			Done:            runAttempt.Status.IsDone(),
			Link:            getRunViewLink(run, runAttempt),
			Current:         runAttempt.ID == attempt.ID,
			Latest:          runAttempt.ID == run.LatestAttemptID,
			TriggeredAt:     runAttempt.Created.AsTime().Unix(),
			TriggerUserName: runAttempt.TriggerUser.GetDisplayName(),
			TriggerUserLink: runAttempt.TriggerUser.HomeLink(),
		})
	}

	pusher := ViewUser{
		DisplayName: run.TriggerUser.GetDisplayName(),
		Link:        run.TriggerUser.HomeLink(),
	}
	branch := ViewBranch{
		Name: run.PrettyRef(),
		Link: run.RefLink(),
	}
	refName := git.RefName(run.Ref)
	if refName.IsBranch() {
		b, err := git_model.GetBranch(ctx, ctx.Repo.Repository.ID, refName.ShortName())
		if err != nil && !git_model.IsErrBranchNotExist(err) {
			log.Error("GetBranch: %v", err)
		} else if git_model.IsErrBranchNotExist(err) || (b != nil && b.IsDeleted) {
			branch.IsDeleted = true
		}
	}

	resp.State.Run.Commit = ViewCommit{
		ShortSha: base.ShortSha(run.CommitSHA),
		Link:     fmt.Sprintf("%s/commit/%s", run.Repo.Link(), run.CommitSHA),
		Pusher:   pusher,
		Branch:   branch,
	}
	resp.State.Run.TriggerEvent = run.TriggerEvent

	// Legacy runs (LatestAttemptID == 0) have no attempt; their artifacts all share run_attempt_id=0,
	// so passing 0 here scopes to this run's legacy artifacts only.
	var runAttemptID int64
	if attempt != nil {
		runAttemptID = attempt.ID
	}
	arts, err := actions_model.ListUploadedArtifactsMetaByRunAttempt(ctx, ctx.Repo.Repository.ID, run.ID, runAttemptID)
	if err != nil {
		ctx.ServerError("ListUploadedArtifactsMetaByRunAttempt", err)
		return
	}
	resp.Artifacts = make([]*ArtifactsViewItem, 0, len(arts))
	for _, art := range arts {
		resp.Artifacts = append(resp.Artifacts, &ArtifactsViewItem{
			Name:   art.ArtifactName,
			Size:   art.FileSize,
			Status: util.Iif(art.Status == actions_model.ArtifactStatusExpired, "expired", "completed"),
		})
	}
}

func fillViewRunResponseCurrentJob(ctx *context_module.Context, resp *ViewResponse, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) {
	req := web.GetForm(ctx).(*ViewRequest)
	current, hasPathParam := findCurrentJobByPathParam(ctx, jobs)
	if current == nil {
		if hasPathParam {
			ctx.NotFound(nil)
		}
		return
	}

	var task *actions_model.ActionTask
	if effectiveTaskID := current.EffectiveTaskID(); effectiveTaskID > 0 {
		var err error
		task, err = actions_model.GetTaskByID(ctx, effectiveTaskID)
		if err != nil {
			ctx.ServerError("actions_model.GetTaskByID", err)
			return
		}
		task.Job = current
		if err := task.LoadAttributes(ctx); err != nil {
			ctx.ServerError("task.LoadAttributes", err)
			return
		}
	}

	resp.State.CurrentJob.Title = current.Name
	resp.State.CurrentJob.Detail = current.Status.LocaleString(ctx.Locale)
	if run.NeedApproval {
		resp.State.CurrentJob.Detail = ctx.Locale.TrString("actions.need_approval_desc")
	}
	resp.State.CurrentJob.Steps = make([]*ViewJobStep, 0) // marshal to '[]' instead fo 'null' in json
	resp.Logs.StepsLog = make([]*ViewStepLog, 0)          // marshal to '[]' instead fo 'null' in json
	if task != nil {
		steps, logs, err := convertToViewModel(ctx, ctx.Locale, req.LogCursors, task)
		if err != nil {
			ctx.ServerError("convertToViewModel", err)
			return
		}
		resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, steps...)
		resp.Logs.StepsLog = append(resp.Logs.StepsLog, logs...)
	}
}

func convertToViewModel(ctx context.Context, locale translation.Locale, cursors []LogCursor, task *actions_model.ActionTask) ([]*ViewJobStep, []*ViewStepLog, error) {
	var viewJobs []*ViewJobStep
	var logs []*ViewStepLog

	steps := actions.FullSteps(task)

	for _, v := range steps {
		viewJobs = append(viewJobs, &ViewJobStep{
			Summary:  v.Name,
			Duration: v.Duration().String(),
			Status:   v.Status.String(),
		})
	}

	for _, cursor := range cursors {
		if !cursor.Expanded {
			continue
		}

		step := steps[cursor.Step]

		// if task log is expired, return a consistent log line
		if task.LogExpired {
			if cursor.Cursor == 0 {
				logs = append(logs, &ViewStepLog{
					Step:   cursor.Step,
					Cursor: 1,
					Lines: []*ViewStepLogLine{
						{
							Index:   1,
							Message: locale.TrString("actions.runs.expire_log_message"),
							// Timestamp doesn't mean anything when the log is expired.
							// Set it to the task's updated time since it's probably the time when the log has expired.
							Timestamp: float64(task.Updated.AsTime().UnixNano()) / float64(time.Second),
						},
					},
					Started: int64(step.Started),
				})
			}
			continue
		}

		logLines := make([]*ViewStepLogLine, 0) // marshal to '[]' instead fo 'null' in json

		index := step.LogIndex + cursor.Cursor
		validCursor := cursor.Cursor >= 0 &&
			// !(cursor.Cursor < step.LogLength) when the frontend tries to fetch next line before it's ready.
			// So return the same cursor and empty lines to let the frontend retry.
			cursor.Cursor < step.LogLength &&
			// !(index < task.LogIndexes[index]) when task data is older than step data.
			// It can be fixed by making sure write/read tasks and steps in the same transaction,
			// but it's easier to just treat it as fetching the next line before it's ready.
			index < int64(len(task.LogIndexes))

		if validCursor {
			length := step.LogLength - cursor.Cursor
			offset := task.LogIndexes[index]
			logRows, err := actions.ReadLogs(ctx, task.LogInStorage, task.LogFilename, offset, length)
			if err != nil {
				return nil, nil, fmt.Errorf("actions.ReadLogs: %w", err)
			}

			for i, row := range logRows {
				logLines = append(logLines, &ViewStepLogLine{
					Index:     cursor.Cursor + int64(i) + 1, // start at 1
					Message:   row.Content,
					Timestamp: float64(row.Time.AsTime().UnixNano()) / float64(time.Second),
				})
			}
		}

		logs = append(logs, &ViewStepLog{
			Step:    cursor.Step,
			Cursor:  cursor.Cursor + int64(len(logLines)),
			Lines:   logLines,
			Started: int64(step.Started),
		})
	}

	return viewJobs, logs, nil
}

// checkRunRerunAllowed checks whether a rerun is permitted for the given run,
// writing the appropriate JSON error to ctx and returning false when it is not.
func checkRunRerunAllowed(ctx *context_module.Context, run *actions_model.ActionRun) bool {
	if !run.Status.IsDone() {
		ctx.JSONError(ctx.Locale.Tr("actions.runs.not_done"))
		return false
	}
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(run.WorkflowID) {
		ctx.JSONError(ctx.Locale.Tr("actions.workflow.disabled"))
		return false
	}
	return true
}

func checkLatestAttempt(ctx *context_module.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt) bool {
	if attempt != nil && run.LatestAttemptID != attempt.ID {
		ctx.NotFound(nil)
		return false
	}
	return true
}

// Rerun will rerun jobs in the given run
// If jobIDStr is a blank string, it means rerun all jobs
func Rerun(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if !checkLatestAttempt(ctx, run, attempt) {
		return
	}
	if !checkRunRerunAllowed(ctx, run) {
		return
	}

	currentJob, hasPathParam := findCurrentJobByPathParam(ctx, jobs)
	if hasPathParam && currentJob == nil {
		ctx.NotFound(nil)
		return
	}

	var jobsToRerun []*actions_model.ActionRunJob
	if currentJob != nil {
		jobsToRerun = []*actions_model.ActionRunJob{currentJob}
	}

	if _, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, jobsToRerun); err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	ctx.JSONRedirect(run.Link())
}

// RerunFailed reruns all failed jobs in the given run
func RerunFailed(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if !checkLatestAttempt(ctx, run, attempt) {
		return
	}
	if !checkRunRerunAllowed(ctx, run) {
		return
	}

	if _, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, actions_service.GetFailedJobsForRerun(jobs)); err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	ctx.JSONRedirect(run.Link())
}

func handleWorkflowRerunError(ctx *context_module.Context, err error) {
	if errors.Is(err, util.ErrAlreadyExist) {
		ctx.JSON(http.StatusConflict, map[string]any{"message": err.Error()})
		return
	}
	if errors.Is(err, util.ErrInvalidArgument) {
		ctx.JSON(http.StatusBadRequest, map[string]any{"message": err.Error()})
		return
	}
	ctx.ServerError("RerunWorkflowRunJobs", err)
}

func Logs(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	jobID := ctx.PathParamInt64("job")

	if err := common.DownloadActionsRunJobLogsWithID(ctx.Base, ctx.Repo.Repository, run.ID, jobID); err != nil {
		ctx.NotFoundOrServerError("DownloadActionsRunJobLogsWithID", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
	}
}

func Cancel(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if !checkLatestAttempt(ctx, run, attempt) {
		return
	}

	var updatedJobs []*actions_model.ActionRunJob

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		cancelledJobs, err := actions_model.CancelJobs(ctx, jobs)
		if err != nil {
			return fmt.Errorf("cancel jobs: %w", err)
		}
		updatedJobs = append(updatedJobs, cancelledJobs...)
		return nil
	}); err != nil {
		ctx.ServerError("StopTask", err)
		return
	}

	actions_service.CreateCommitStatusForRunJobs(ctx, run, jobs...)
	actions_service.EmitJobsIfReadyByJobs(updatedJobs)

	actions_service.NotifyWorkflowJobsStatusUpdate(ctx, updatedJobs...)
	if len(updatedJobs) > 0 {
		actions_service.NotifyWorkflowRunStatusUpdateWithReload(ctx, run.RepoID, run.ID)
	}
	ctx.JSONOK()
}

func Approve(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if err := actions_service.ApproveRuns(ctx, ctx.Repo.Repository, ctx.Doer, []int64{run.ID}); err != nil {
		ctx.NotFoundOrServerError("ApproveRuns", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}

	ctx.JSONOK()
}

func Delete(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}

	if !run.Status.IsDone() {
		ctx.JSONError(ctx.Tr("actions.runs.not_done"))
		return
	}

	if err := actions_service.DeleteRun(ctx, run); err != nil {
		ctx.ServerError("DeleteRun", err)
		return
	}

	ctx.JSONOK()
}

func getRunViewLink(run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt) string {
	if attempt == nil || run.LatestAttemptID == attempt.ID {
		return run.Link()
	}
	return fmt.Sprintf("%s/attempts/%d", run.Link(), attempt.Attempt)
}

// getCurrentRunJobsByPathParam resolves the current run view context from path parameters, including the run, optional attempt, and jobs to render.
// Any error will be written to the ctx, empty jobs will also result in 404 error, then the return values are all nil.
func getCurrentRunJobsByPathParam(ctx *context_module.Context) (*actions_model.ActionRun, *actions_model.ActionRunAttempt, []*actions_model.ActionRunJob) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return nil, nil, nil
	}
	run.Repo = ctx.Repo.Repository

	var err error
	var selectedJob *actions_model.ActionRunJob
	if ctx.PathParam("job") != "" {
		jobID := ctx.PathParamInt64("job")
		selectedJob, err = actions_model.GetRunJobByRunAndID(ctx, run.ID, jobID)
		if err != nil {
			ctx.NotFoundOrServerError("GetRunJobByRepoAndID", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	}

	// Resolve the attempt to display.
	// Priority: explicit path param (/attempts/:num) > job's attempt (when navigating to a specific job) > latest attempt.
	// attempt may be nil for legacy runs that pre-date ActionRunAttempt; callers must handle that case.
	attemptNum := ctx.PathParamInt64("attempt")
	var attempt *actions_model.ActionRunAttempt
	switch {
	case attemptNum > 0:
		// Explicit attempt number in the URL — user is viewing a historical attempt.
		attempt, err = actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, attemptNum)
		if err != nil {
			ctx.NotFoundOrServerError("GetRunAttemptByRunIDAndAttempt", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	case selectedJob != nil && selectedJob.RunAttemptID > 0:
		// No explicit attempt in the URL, but the requested job belongs to a known attempt — resolve via the job.
		attempt, err = actions_model.GetRunAttemptByRepoAndID(ctx, selectedJob.RepoID, selectedJob.RunAttemptID)
		if err != nil {
			ctx.NotFoundOrServerError("GetRunAttemptByRepoAndID", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	default:
		// No attempt context at all — show the latest attempt (nil for legacy runs).
		attempt, _, err = run.GetLatestAttempt(ctx)
		if err != nil {
			ctx.NotFoundOrServerError("GetLatestAttempt", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	}

	// Resolve the jobs for the resolved attempt.
	// When attempt is nil (legacy run or legacy job), jobs are stored with run_attempt_id=0.
	var resolvedAttemptID int64
	if attempt != nil {
		resolvedAttemptID = attempt.ID
	}
	jobs, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, resolvedAttemptID)
	if err != nil {
		ctx.ServerError("get current jobs", err)
		return nil, nil, nil
	}
	if len(jobs) == 0 {
		ctx.NotFound(nil)
		return nil, nil, nil
	}

	for _, job := range jobs {
		job.Run = run
	}
	return run, attempt, jobs
}

// resolveArtifactAttemptIDFromQuery resolves the run_attempt_id used to scope artifact lookups.
// If the `attempt` query parameter is present and valid, it returns the matching attempt's ID.
// Otherwise it falls back to run.LatestAttemptID, which is 0 only for legacy runs created before ActionRunAttempt existed.
func resolveArtifactAttemptIDFromQuery(ctx *context_module.Context, run *actions_model.ActionRun) (int64, error) {
	if ctx.FormString("attempt") == "" {
		return run.LatestAttemptID, nil
	}
	attemptNum := ctx.FormInt64("attempt")
	if attemptNum <= 0 {
		return 0, util.ErrNotExist
	}
	attempt, err := actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, attemptNum)
	if err != nil {
		return 0, err
	}
	return attempt.ID, nil
}

func ArtifactsDeleteView(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	resolvedAttemptID, err := resolveArtifactAttemptIDFromQuery(ctx, run)
	if err != nil {
		ctx.NotFoundOrServerError("resolveArtifactAttemptIDFromQuery", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}
	artifactName := ctx.PathParam("artifact_name")
	if err := actions_model.SetArtifactNeedDeleteByRunAttempt(ctx, run.ID, resolvedAttemptID, artifactName); err != nil {
		ctx.ServerError("SetArtifactNeedDeleteByRunAttempt", err)
		return
	}
	ctx.JSON(http.StatusOK, struct{}{})
}

func ArtifactsDownloadView(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	resolvedAttemptID, err := resolveArtifactAttemptIDFromQuery(ctx, run)
	if err != nil {
		ctx.NotFoundOrServerError("resolveArtifactAttemptIDFromQuery", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}
	artifactName := ctx.PathParam("artifact_name")
	artifacts, err := actions_model.GetArtifactsByRunAttemptAndName(ctx, run.ID, resolvedAttemptID, artifactName)
	if err != nil {
		ctx.ServerError("GetArtifactsByRunAttemptAndName", err)
		return
	}
	if len(artifacts) == 0 {
		ctx.HTTPError(http.StatusNotFound, "artifact not found")
		return
	}

	// if artifacts status is not uploaded-confirmed, treat it as not found
	for _, art := range artifacts {
		if art.Status != actions_model.ArtifactStatusUploadConfirmed {
			ctx.HTTPError(http.StatusNotFound, "artifact not found")
			return
		}
	}

	// A v4 Artifact may only contain a single file
	// Multiple files are uploaded as a single file archive
	// All other cases fall back to the legacy v1–v3 zip handling below
	if len(artifacts) == 1 && actions.IsArtifactV4(artifacts[0]) {
		err := actions.DownloadArtifactV4(ctx.Base, artifacts[0])
		if err != nil {
			ctx.ServerError("DownloadArtifactV4", err)
			return
		}
		return
	}

	// Artifacts using the v1-v3 backend are stored as multiple individual files per artifact on the backend
	// Those need to be zipped for download
	ctx.Resp.Header().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(artifactName+".zip"))
	zipWriter := zip.NewWriter(ctx.Resp)
	defer zipWriter.Close()

	writeArtifactToZip := func(art *actions_model.ActionArtifact) error {
		f, err := storage.ActionsArtifacts.Open(art.StoragePath)
		if err != nil {
			return fmt.Errorf("ActionsArtifacts.Open: %w", err)
		}
		defer f.Close()

		var r io.ReadCloser = f
		if art.ContentEncodingOrType == actions_model.ContentEncodingV3Gzip {
			r, err = gzip.NewReader(f)
			if err != nil {
				return fmt.Errorf("gzip.NewReader: %w", err)
			}
		}
		defer r.Close()

		w, err := zipWriter.Create(art.ArtifactPath)
		if err != nil {
			return fmt.Errorf("zipWriter.Create: %w", err)
		}
		_, err = io.Copy(w, r)
		if err != nil {
			return fmt.Errorf("io.Copy: %w", err)
		}
		return nil
	}

	for _, art := range artifacts {
		err := writeArtifactToZip(art)
		if err != nil {
			ctx.ServerError("writeArtifactToZip", err)
			return
		}
	}
}

func ApproveAllChecks(ctx *context_module.Context) {
	repo := ctx.Repo.Repository
	commitID := ctx.FormString("commit_id")

	commitStatuses, err := git_model.GetLatestCommitStatus(ctx, repo.ID, commitID, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetLatestCommitStatus", err)
		return
	}
	runs, err := actions_service.GetRunsFromCommitStatuses(ctx, commitStatuses)
	if err != nil {
		ctx.ServerError("GetRunsFromCommitStatuses", err)
		return
	}

	runIDs := make([]int64, 0, len(runs))
	for _, run := range runs {
		if run.NeedApproval {
			runIDs = append(runIDs, run.ID)
		}
	}

	if len(runIDs) == 0 {
		ctx.JSONOK()
		return
	}

	if err := actions_service.ApproveRuns(ctx, repo, ctx.Doer, runIDs); err != nil {
		ctx.NotFoundOrServerError("ApproveRuns", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.approve_all_success"))
	ctx.JSONOK()
}

func DisableWorkflowFile(ctx *context_module.Context) {
	disableOrEnableWorkflowFile(ctx, false)
}

func EnableWorkflowFile(ctx *context_module.Context) {
	disableOrEnableWorkflowFile(ctx, true)
}

func disableOrEnableWorkflowFile(ctx *context_module.Context, isEnable bool) {
	workflow := ctx.FormString("workflow")
	if len(workflow) == 0 {
		ctx.JSONError("workflow is required")
		return
	}

	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	if isEnable {
		cfg.EnableWorkflow(workflow)
	} else {
		cfg.DisableWorkflow(workflow)
	}

	if err := repo_model.UpdateRepoUnitConfig(ctx, cfgUnit); err != nil {
		ctx.ServerError("UpdateRepoUnit", err)
		return
	}

	if isEnable {
		ctx.Flash.Success(ctx.Tr("actions.workflow.enable_success", workflow))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.workflow.disable_success", workflow))
	}

	redirectURL := fmt.Sprintf("%s/actions?workflow=%s&actor=%s&status=%s", ctx.Repo.RepoLink, url.QueryEscape(workflow),
		url.QueryEscape(ctx.FormString("actor")), url.QueryEscape(ctx.FormString("status")))
	ctx.JSONRedirect(redirectURL)
}

func Run(ctx *context_module.Context) {
	redirectURL := fmt.Sprintf("%s/actions?workflow=%s&actor=%s&status=%s", ctx.Repo.RepoLink, url.QueryEscape(ctx.FormString("workflow")),
		url.QueryEscape(ctx.FormString("actor")), url.QueryEscape(ctx.FormString("status")))

	workflowID := ctx.FormString("workflow")
	if len(workflowID) == 0 {
		ctx.ServerError("workflow", nil)
		return
	}

	ref := ctx.FormString("ref")
	if len(ref) == 0 {
		ctx.ServerError("ref", nil)
		return
	}
	_, err := actions_service.DispatchActionWorkflow(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, workflowID, ref, func(workflowDispatch *model.WorkflowDispatch, inputs map[string]any) error {
		for name, config := range workflowDispatch.Inputs {
			value := ctx.Req.PostFormValue(name)
			if config.Type == "boolean" {
				inputs[name] = strconv.FormatBool(ctx.FormBool(name))
			} else if value != "" {
				inputs[name] = value
			} else {
				inputs[name] = config.Default
			}
		}
		return nil
	})
	if err != nil {
		if errTr := util.ErrorAsTranslatable(err); errTr != nil {
			ctx.Flash.Error(errTr.Translate(ctx.Locale))
			ctx.Redirect(redirectURL)
		} else {
			ctx.ServerError("DispatchActionWorkflow", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.workflow.run_success", workflowID))
	ctx.Redirect(redirectURL)
}
