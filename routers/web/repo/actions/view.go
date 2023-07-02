// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/base"
	context_module "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	actions_service "code.gitea.io/gitea/services/actions"

	"xorm.io/builder"
)

func View(ctx *context_module.Context) {
	ctx.Data["PageIsActions"] = true
	runIndex := ctx.ParamsInt64("run")
	jobIndex := ctx.ParamsInt64("job")
	ctx.Data["RunIndex"] = runIndex
	ctx.Data["JobIndex"] = jobIndex
	ctx.Data["ActionsURL"] = ctx.Repo.RepoLink + "/actions"

	if getRunJobs(ctx, runIndex, jobIndex); ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplViewActions)
}

type ViewRequest struct {
	LogCursors []struct {
		Step     int   `json:"step"`
		Cursor   int64 `json:"cursor"`
		Expanded bool  `json:"expanded"`
	} `json:"logCursors"`
}

type ViewResponse struct {
	State struct {
		Run struct {
			Link       string     `json:"link"`
			Title      string     `json:"title"`
			Status     string     `json:"status"`
			CanCancel  bool       `json:"canCancel"`
			CanApprove bool       `json:"canApprove"` // the run needs an approval and the doer has permission to approve
			CanRerun   bool       `json:"canRerun"`
			Done       bool       `json:"done"`
			Jobs       []*ViewJob `json:"jobs"`
			Commit     ViewCommit `json:"commit"`
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
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	CanRerun bool   `json:"canRerun"`
	Duration string `json:"duration"`
}

type ViewCommit struct {
	LocaleCommit   string     `json:"localeCommit"`
	LocalePushedBy string     `json:"localePushedBy"`
	ShortSha       string     `json:"shortSHA"`
	Link           string     `json:"link"`
	Pusher         ViewUser   `json:"pusher"`
	Branch         ViewBranch `json:"branch"`
}

type ViewUser struct {
	DisplayName string `json:"displayName"`
	Link        string `json:"link"`
}

type ViewBranch struct {
	Name string `json:"name"`
	Link string `json:"link"`
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
	req := web.GetForm(ctx).(*ViewRequest)
	runIndex := ctx.ParamsInt64("run")
	jobIndex := ctx.ParamsInt64("job")

	current, jobs := getRunJobs(ctx, runIndex, jobIndex)
	if ctx.Written() {
		return
	}
	run := current.Run
	if err := run.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	resp := &ViewResponse{}

	resp.State.Run.Title = run.Title
	resp.State.Run.Link = run.Link()
	resp.State.Run.CanCancel = !run.Status.IsDone() && ctx.Repo.CanWrite(unit.TypeActions)
	resp.State.Run.CanApprove = run.NeedApproval && ctx.Repo.CanWrite(unit.TypeActions)
	resp.State.Run.CanRerun = run.Status.IsDone() && ctx.Repo.CanWrite(unit.TypeActions)
	resp.State.Run.Done = run.Status.IsDone()
	resp.State.Run.Jobs = make([]*ViewJob, 0, len(jobs)) // marshal to '[]' instead fo 'null' in json
	resp.State.Run.Status = run.Status.String()
	for _, v := range jobs {
		resp.State.Run.Jobs = append(resp.State.Run.Jobs, &ViewJob{
			ID:       v.ID,
			Name:     v.Name,
			Status:   v.Status.String(),
			CanRerun: v.Status.IsDone() && ctx.Repo.CanWrite(unit.TypeActions),
			Duration: v.Duration().String(),
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
	resp.State.Run.Commit = ViewCommit{
		LocaleCommit:   ctx.Tr("actions.runs.commit"),
		LocalePushedBy: ctx.Tr("actions.runs.pushed_by"),
		ShortSha:       base.ShortSha(run.CommitSHA),
		Link:           fmt.Sprintf("%s/commit/%s", run.Repo.Link(), run.CommitSHA),
		Pusher:         pusher,
		Branch:         branch,
	}

	var task *actions_model.ActionTask
	if current.TaskID > 0 {
		var err error
		task, err = actions_model.GetTaskByID(ctx, current.TaskID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		task.Job = current
		if err := task.LoadAttributes(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	resp.State.CurrentJob.Title = current.Name
	resp.State.CurrentJob.Detail = current.Status.LocaleString(ctx.Locale)
	if run.NeedApproval {
		resp.State.CurrentJob.Detail = ctx.Locale.Tr("actions.need_approval_desc")
	}
	resp.State.CurrentJob.Steps = make([]*ViewJobStep, 0) // marshal to '[]' instead fo 'null' in json
	resp.Logs.StepsLog = make([]*ViewStepLog, 0)          // marshal to '[]' instead fo 'null' in json
	if task != nil {
		steps := actions.FullSteps(task)

		for _, v := range steps {
			resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &ViewJobStep{
				Summary:  v.Name,
				Duration: v.Duration().String(),
				Status:   v.Status.String(),
			})
		}

		for _, cursor := range req.LogCursors {
			if !cursor.Expanded {
				continue
			}

			step := steps[cursor.Step]

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
				var err error
				logRows, err := actions.ReadLogs(ctx, task.LogInStorage, task.LogFilename, offset, length)
				if err != nil {
					ctx.Error(http.StatusInternalServerError, err.Error())
					return
				}

				for i, row := range logRows {
					logLines = append(logLines, &ViewStepLogLine{
						Index:     cursor.Cursor + int64(i) + 1, // start at 1
						Message:   row.Content,
						Timestamp: float64(row.Time.AsTime().UnixNano()) / float64(time.Second),
					})
				}
			}

			resp.Logs.StepsLog = append(resp.Logs.StepsLog, &ViewStepLog{
				Step:    cursor.Step,
				Cursor:  cursor.Cursor + int64(len(logLines)),
				Lines:   logLines,
				Started: int64(step.Started),
			})
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func RerunOne(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")
	jobIndex := ctx.ParamsInt64("job")

	job, _ := getRunJobs(ctx, runIndex, jobIndex)
	if ctx.Written() {
		return
	}

	if err := rerunJob(ctx, job); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, struct{}{})
}

func RerunAll(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")

	_, jobs := getRunJobs(ctx, runIndex, 0)
	if ctx.Written() {
		return
	}

	for _, j := range jobs {
		if err := rerunJob(ctx, j); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	ctx.JSON(http.StatusOK, struct{}{})
}

func rerunJob(ctx *context_module.Context, job *actions_model.ActionRunJob) error {
	status := job.Status
	if !status.IsDone() {
		return nil
	}

	job.TaskID = 0
	job.Status = actions_model.StatusWaiting
	job.Started = 0
	job.Stopped = 0

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		_, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": status}, "task_id", "status", "started", "stopped")
		return err
	}); err != nil {
		return err
	}

	actions_service.CreateCommitStatus(ctx, job)
	return nil
}

func Logs(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")
	jobIndex := ctx.ParamsInt64("job")

	job, _ := getRunJobs(ctx, runIndex, jobIndex)
	if ctx.Written() {
		return
	}
	if job.TaskID == 0 {
		ctx.Error(http.StatusNotFound, "job is not started")
		return
	}

	err := job.LoadRun(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	task, err := actions_model.GetTaskByID(ctx, job.TaskID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if task.LogExpired {
		ctx.Error(http.StatusNotFound, "logs have been cleaned up")
		return
	}

	reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	workflowName := job.Run.WorkflowID
	if p := strings.Index(workflowName, "."); p > 0 {
		workflowName = workflowName[0:p]
	}
	ctx.ServeContent(reader, &context_module.ServeHeaderOptions{
		Filename:           fmt.Sprintf("%v-%v-%v.log", workflowName, job.Name, task.ID),
		ContentLength:      &task.LogSize,
		ContentType:        "text/plain",
		ContentTypeCharset: "utf-8",
		Disposition:        "attachment",
	})
}

func Cancel(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")

	_, jobs := getRunJobs(ctx, runIndex, -1)
	if ctx.Written() {
		return
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		for _, job := range jobs {
			status := job.Status
			if status.IsDone() {
				continue
			}
			if job.TaskID == 0 {
				job.Status = actions_model.StatusCancelled
				job.Stopped = timeutil.TimeStampNow()
				n, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"task_id": 0}, "status", "stopped")
				if err != nil {
					return err
				}
				if n == 0 {
					return fmt.Errorf("job has changed, try again")
				}
				continue
			}
			if err := actions_model.StopTask(ctx, job.TaskID, actions_model.StatusCancelled); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	actions_service.CreateCommitStatus(ctx, jobs...)

	ctx.JSON(http.StatusOK, struct{}{})
}

func Approve(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")

	current, jobs := getRunJobs(ctx, runIndex, -1)
	if ctx.Written() {
		return
	}
	run := current.Run
	doer := ctx.Doer

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		run.NeedApproval = false
		run.ApprovedBy = doer.ID
		if err := actions_model.UpdateRun(ctx, run, "need_approval", "approved_by"); err != nil {
			return err
		}
		for _, job := range jobs {
			if len(job.Needs) == 0 && job.Status.IsBlocked() {
				job.Status = actions_model.StatusWaiting
				_, err := actions_model.UpdateRunJob(ctx, job, nil, "status")
				if err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	actions_service.CreateCommitStatus(ctx, jobs...)

	ctx.JSON(http.StatusOK, struct{}{})
}

// getRunJobs gets the jobs of runIndex, and returns jobs[jobIndex], jobs.
// Any error will be written to the ctx.
// It never returns a nil job of an empty jobs, if the jobIndex is out of range, it will be treated as 0.
func getRunJobs(ctx *context_module.Context, runIndex, jobIndex int64) (*actions_model.ActionRunJob, []*actions_model.ActionRunJob) {
	run, err := actions_model.GetRunByIndex(ctx, ctx.Repo.Repository.ID, runIndex)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, err.Error())
			return nil, nil
		}
		ctx.Error(http.StatusInternalServerError, err.Error())
		return nil, nil
	}
	run.Repo = ctx.Repo.Repository

	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return nil, nil
	}
	if len(jobs) == 0 {
		ctx.Error(http.StatusNotFound, err.Error())
		return nil, nil
	}

	for _, v := range jobs {
		v.Run = run
	}

	if jobIndex >= 0 && jobIndex < int64(len(jobs)) {
		return jobs[jobIndex], jobs
	}
	return jobs[0], jobs
}

type ArtifactsViewResponse struct {
	Artifacts []*ArtifactsViewItem `json:"artifacts"`
}

type ArtifactsViewItem struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	ID   int64  `json:"id"`
}

func ArtifactsView(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")
	run, err := actions_model.GetRunByIndex(ctx, ctx.Repo.Repository.ID, runIndex)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, err.Error())
			return
		}
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	artifacts, err := actions_model.ListUploadedArtifactsByRunID(ctx, run.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	artifactsResponse := ArtifactsViewResponse{
		Artifacts: make([]*ArtifactsViewItem, 0, len(artifacts)),
	}
	for _, art := range artifacts {
		artifactsResponse.Artifacts = append(artifactsResponse.Artifacts, &ArtifactsViewItem{
			Name: art.ArtifactName,
			Size: art.FileSize,
			ID:   art.ID,
		})
	}
	ctx.JSON(http.StatusOK, artifactsResponse)
}

func ArtifactsDownloadView(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")
	artifactID := ctx.ParamsInt64("id")

	artifact, err := actions_model.GetArtifactByID(ctx, artifactID)
	if errors.Is(err, util.ErrNotExist) {
		ctx.Error(http.StatusNotFound, err.Error())
	} else if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	run, err := actions_model.GetRunByIndex(ctx, ctx.Repo.Repository.ID, runIndex)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, err.Error())
			return
		}
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if artifact.RunID != run.ID {
		ctx.Error(http.StatusNotFound, "artifact not found")
		return
	}

	f, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()

	ctx.ServeContent(f, &context_module.ServeHeaderOptions{
		Filename:     artifact.ArtifactName,
		LastModified: artifact.CreatedUnix.AsLocalTime(),
	})
}
