// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	context_module "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	actions_service "code.gitea.io/gitea/services/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"xorm.io/builder"
)

func View(ctx *context_module.Context) {
	ctx.Data["PageIsActions"] = true
	runIndex := ctx.ParamsInt64("run")
	jobIndex := ctx.ParamsInt64("job")
	ctx.Data["RunIndex"] = runIndex
	ctx.Data["JobIndex"] = jobIndex

	job, _ := getRunJobs(ctx, runIndex, jobIndex)
	if ctx.Written() {
		return
	}
	run := job.Run
	ctx.Data["Build"] = run

	ctx.HTML(http.StatusOK, tplViewActions)
}

type ViewRequest struct {
	StepLogCursors []struct {
		StepIndex int   `json:"stepIndex"`
		Cursor    int64 `json:"cursor"`
		Expanded  bool  `json:"expanded"`
	} `json:"stepLogCursors"`
}

type ViewResponse struct {
	StateData struct {
		RunInfo struct {
			HTMLURL   string `json:"htmlurl"`
			Title     string `json:"title"`
			CanCancel bool   `json:"can_cancel"`
		} `json:"runInfo"`
		AllJobGroups   []ViewGroup `json:"allJobGroups"`
		CurrentJobInfo struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"currentJobInfo"`
		CurrentJobSteps []ViewJobStep `json:"currentJobSteps"`
	} `json:"stateData"`
	LogsData struct {
		StreamingLogs []ViewStepLog `json:"streamingLogs"`
	} `json:"logsData"`
}

type ViewGroup struct {
	Summary string     `json:"summary"`
	Jobs    []*ViewJob `json:"jobs"`
}

type ViewJob struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	CanRerun bool   `json:"can_rerun"`
}

type ViewJobStep struct {
	Summary  string  `json:"summary"`
	Duration float64 `json:"duration"`
	Status   string  `json:"status"`
}

type ViewStepLog struct {
	StepIndex int               `json:"stepIndex"`
	Cursor    int64             `json:"cursor"`
	Lines     []ViewStepLogLine `json:"lines"`
}

type ViewStepLogLine struct {
	Ln int64   `json:"ln"`
	M  string  `json:"m"`
	T  float64 `json:"t"`
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

	resp := &ViewResponse{}
	resp.StateData.RunInfo.Title = run.Title
	resp.StateData.RunInfo.HTMLURL = run.HTMLURL()
	resp.StateData.RunInfo.CanCancel = !run.Status.IsDone() && ctx.Repo.CanWrite(unit.TypeActions)

	respJobs := make([]*ViewJob, len(jobs))
	for i, v := range jobs {
		respJobs[i] = &ViewJob{
			ID:       v.ID,
			Name:     v.Name,
			Status:   v.Status.String(),
			CanRerun: v.Status.IsDone() && ctx.Repo.CanWrite(unit.TypeActions),
		}
	}

	resp.StateData.AllJobGroups = []ViewGroup{
		{
			Jobs: respJobs,
		},
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

	resp.StateData.CurrentJobInfo.Title = current.Name
	resp.StateData.CurrentJobSteps = make([]ViewJobStep, 0)
	resp.LogsData.StreamingLogs = make([]ViewStepLog, 0, len(req.StepLogCursors))
	resp.StateData.CurrentJobInfo.Detail = current.Status.LocaleString(ctx.Locale)
	if task != nil {
		steps := actions.FullSteps(task)

		resp.StateData.CurrentJobSteps = make([]ViewJobStep, len(steps))
		for i, v := range steps {
			resp.StateData.CurrentJobSteps[i] = ViewJobStep{
				Summary:  v.Name,
				Duration: float64(v.Duration() / time.Second),
				Status:   v.Status.String(),
			}
		}

		for _, cursor := range req.StepLogCursors {
			if cursor.Expanded {
				step := steps[cursor.StepIndex]
				var logRows []*runnerv1.LogRow
				if cursor.Cursor < step.LogLength || step.LogLength < 0 {
					index := step.LogIndex + cursor.Cursor
					length := step.LogLength - cursor.Cursor
					offset := (*task.LogIndexes)[index]
					var err error
					logRows, err = actions.ReadLogs(ctx, task.LogInStorage, task.LogFilename, offset, length)
					if err != nil {
						ctx.Error(http.StatusInternalServerError, err.Error())
						return
					}
				}
				logLines := make([]ViewStepLogLine, len(logRows))
				for i, row := range logRows {
					logLines[i] = ViewStepLogLine{
						Ln: cursor.Cursor + int64(i) + 1, // start at 1
						M:  row.Content,
						T:  float64(row.Time.AsTime().UnixNano()) / float64(time.Second),
					}
				}
				resp.LogsData.StreamingLogs = append(resp.LogsData.StreamingLogs, ViewStepLog{
					StepIndex: cursor.StepIndex,
					Cursor:    cursor.Cursor + int64(len(logLines)),
					Lines:     logLines,
				})
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func Rerun(ctx *context_module.Context) {
	runIndex := ctx.ParamsInt64("run")
	jobIndex := ctx.ParamsInt64("job")

	job, _ := getRunJobs(ctx, runIndex, jobIndex)
	if ctx.Written() {
		return
	}
	status := job.Status
	if !status.IsDone() {
		ctx.JSON(http.StatusOK, struct{}{})
		return
	}

	job.TaskID = 0
	job.Status = actions_model.StatusWaiting
	job.Started = 0
	job.Stopped = 0

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": status}, "task_id", "status", "started", "stopped"); err != nil {
			return err
		}
		return actions_service.CreateCommitStatus(ctx, job)
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, struct{}{})
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
			if err := actions_service.CreateCommitStatus(ctx, job); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

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
