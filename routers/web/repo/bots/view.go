// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"
	"net/http"
	"time"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/bots"
	context_module "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"

	runnerv1 "code.gitea.io/bots-proto-go/runner/v1"
	"xorm.io/builder"
)

func View(ctx *context_module.Context) {
	ctx.Data["PageIsBots"] = true
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

	ctx.HTML(http.StatusOK, tplViewBuild)
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
		BuildInfo struct {
			HTMLURL    string `json:"htmlurl"`
			Title      string `json:"title"`
			Cancelable bool   `json:"cancelable"`
		} `json:"buildInfo"`
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
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
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
	resp.StateData.BuildInfo.Title = run.Title
	resp.StateData.BuildInfo.HTMLURL = run.HTMLURL()
	resp.StateData.BuildInfo.Cancelable = !run.Status.IsDone()

	respJobs := make([]*ViewJob, len(jobs))
	for i, v := range jobs {
		respJobs[i] = &ViewJob{
			ID:     v.ID,
			Name:   v.Name,
			Status: v.Status.String(),
		}
	}

	resp.StateData.AllJobGroups = []ViewGroup{
		{
			Jobs: respJobs,
		},
	}

	if current != nil {
		var task *bots_model.Task
		if current.TaskID > 0 {
			var err error
			task, err = bots_model.GetTaskByID(ctx, current.TaskID)
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
		if task == nil {
			resp.StateData.CurrentJobInfo.Detail = "wait to be pick up by a runner"
		} else {
			resp.StateData.CurrentJobInfo.Detail = "TODO: more detail info" // TODO: more detail info

			steps := bots.FullSteps(task)

			resp.StateData.CurrentJobSteps = make([]ViewJobStep, len(steps))
			for i, v := range steps {
				resp.StateData.CurrentJobSteps[i] = ViewJobStep{
					Summary:  v.Name,
					Duration: float64(v.TakeTime() / time.Second),
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
						logRows, err = bots.ReadLogs(ctx, task.LogInStorage, task.LogFilename, offset, length)
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
	job.Status = bots_model.StatusWaiting
	job.Started = 0
	job.Stopped = 0

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		_, err := bots_model.UpdateRunJob(ctx, job, builder.Eq{"status": status}, "task_id", "status", "started", "stopped")
		return err
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
				job.Status = bots_model.StatusCancelled
				job.Stopped = timeutil.TimeStampNow()
				n, err := bots_model.UpdateRunJob(ctx, job, builder.Eq{"task_id": 0}, "status", "stopped")
				if err != nil {
					return err
				}
				if n == 0 {
					return fmt.Errorf("job has changed, try again")
				}
				continue
			}
			if err := bots_model.StopTask(ctx, job.TaskID, bots_model.StatusCancelled); err != nil {
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

func getRunJobs(ctx *context_module.Context, runIndex, jobIndex int64) (current *bots_model.RunJob, jobs []*bots_model.RunJob) {
	run, err := bots_model.GetRunByIndex(ctx, ctx.Repo.Repository.ID, runIndex)
	if err != nil {
		if _, ok := err.(bots_model.ErrRunNotExist); ok {
			ctx.Error(http.StatusNotFound, err.Error())
			return
		}
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	run.Repo = ctx.Repo.Repository

	jobs, err = bots_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return nil, nil
	}
	for _, v := range jobs {
		v.Run = run
	}

	if jobIndex >= 0 && jobIndex < int64(len(jobs)) {
		if len(jobs) == 0 {
			ctx.Error(http.StatusNotFound, fmt.Sprintf("run %v has no job %v", runIndex, jobIndex))
			return nil, nil
		}
		current = jobs[jobIndex]
	}
	return
}
