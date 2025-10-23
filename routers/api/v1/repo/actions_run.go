// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	stdCtx "context"
	"errors"
	"net/http"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/common"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	notify_service "code.gitea.io/gitea/services/notify"

	"xorm.io/builder"
)

func DownloadActionsRunJobLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs repository downloadActionsRunJobLogs
	// ---
	// summary: Downloads the job logs for a workflow run
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: job_id
	//   in: path
	//   description: id of the job
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: output blob content
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	jobID := ctx.PathParamInt64("job_id")
	curJob, err := actions_model.GetRunJobByID(ctx, jobID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err = curJob.LoadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	err = common.DownloadActionsRunJobLogs(ctx.Base, ctx.Repo.Repository, curJob)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
	}
}

func RerunWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/rerun repository rerunWorkflowRun
	// ---
	// summary: Rerun a workflow run and its jobs
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	_, run, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Check if workflow is disabled
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(run.WorkflowID) {
		ctx.APIError(400, "Workflow is disabled")
		return
	}

	// Reset run's start and stop time when it is done
	if err := actions_service.ResetRunTimes(ctx, run); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Rerun all jobs
	for _, job := range jobs {
		// If the job has needs, it should be set to "blocked" status to wait for other jobs
		shouldBlock := len(job.Needs) > 0
		if err := actions_service.RerunJob(ctx, job, shouldBlock); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		actions_service.NotifyWorkflowRunStatusUpdateWithReload(ctx, job)
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
	}

	ctx.Status(200)
}

func CancelWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/cancel repository cancelWorkflowRun
	// ---
	// summary: Cancel a workflow run and its jobs
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID, _, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	jobs, err := getRunJobsByRunID(ctx, runID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	var updatedJobs []*actions_model.ActionRunJob

	if err := db.WithTx(ctx, func(ctx stdCtx.Context) error {
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
					return errors.New("job has changed, try again")
				}
				if n > 0 {
					updatedJobs = append(updatedJobs, job)
				}
				continue
			}
			if err := actions_model.StopTask(ctx, job.TaskID, actions_model.StatusCancelled); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	actions_service.CreateCommitStatusForRunJobs(ctx, jobs[0].Run, jobs...)

	for _, job := range updatedJobs {
		_ = job.LoadAttributes(ctx)
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
	}
	if len(updatedJobs) > 0 {
		job := updatedJobs[0]
		actions_service.NotifyWorkflowRunStatusUpdateWithReload(ctx, job)
		notify_service.WorkflowRunStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job.Run)
	}

	ctx.Status(200)
}

func ApproveWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/approve repository approveWorkflowRun
	// ---
	// summary: Approve a workflow run that requires approval
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID, _, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	current, jobs, err := getRunJobsAndCurrent(ctx, runID, -1)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	run := current.Run
	doer := ctx.Doer

	var updatedJobs []*actions_model.ActionRunJob

	if err := db.WithTx(ctx, func(ctx stdCtx.Context) error {
		run.NeedApproval = false
		run.ApprovedBy = doer.ID
		if err := actions_model.UpdateRun(ctx, run, "need_approval", "approved_by"); err != nil {
			return err
		}
		for _, job := range jobs {
			if len(job.Needs) == 0 && job.Status.IsBlocked() {
				job.Status = actions_model.StatusWaiting
				n, err := actions_model.UpdateRunJob(ctx, job, nil, "status")
				if err != nil {
					return err
				}
				if n > 0 {
					updatedJobs = append(updatedJobs, job)
				}
			}
		}
		return nil
	}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	actions_service.CreateCommitStatusForRunJobs(ctx, jobs[0].Run, jobs...)

	if len(updatedJobs) > 0 {
		job := updatedJobs[0]
		actions_service.NotifyWorkflowRunStatusUpdateWithReload(ctx, job)
		notify_service.WorkflowRunStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job.Run)
	}

	for _, job := range updatedJobs {
		_ = job.LoadAttributes(ctx)
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
	}

	ctx.Status(200)
}

func RerunWorkflowJob(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/jobs/{job_id}/rerun repository rerunWorkflowJob
	// ---
	// summary: Rerun a specific job and its dependent jobs
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// - name: job_id
	//   in: path
	//   description: id of the job
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID, _, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	jobID := ctx.PathParamInt64("job_id")

	// Get all jobs for the run to handle dependencies
	allJobs, err := getRunJobsByRunID(ctx, runID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Find the specific job in the list
	var job *actions_model.ActionRunJob
	for _, j := range allJobs {
		if j.ID == jobID {
			job = j
			break
		}
	}

	if job == nil {
		ctx.APIError(404, "Job not found in run")
		return
	}

	// Get run from the job and check if workflow is disabled
	run := allJobs[0].Run

	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(run.WorkflowID) {
		ctx.APIError(400, "Workflow is disabled")
		return
	}

	// Reset run's start and stop time when it is done
	if err := actions_service.ResetRunTimes(ctx, run); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Get all jobs that need to be rerun (including dependencies)
	rerunJobs := actions_service.GetAllRerunJobs(job, allJobs)

	for _, j := range rerunJobs {
		// Jobs other than the specified one should be set to "blocked" status
		shouldBlock := j.JobID != job.JobID
		if err := actions_service.RerunJob(ctx, j, shouldBlock); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		actions_service.NotifyWorkflowRunStatusUpdateWithReload(ctx, j)
		notify_service.WorkflowJobStatusUpdate(ctx, j.Run.Repo, j.Run.TriggerUser, j, nil)
	}

	ctx.Status(200)
}

// Helper functions
func getRunID(ctx *context.APIContext) (int64, *actions_model.ActionRun, error) {
	runID := ctx.PathParamInt64("run")
	run, has, err := db.GetByID[actions_model.ActionRun](ctx, runID)
	if err != nil {
		return 0, nil, err
	}
	if !has || run.RepoID != ctx.Repo.Repository.ID {
		return 0, nil, util.ErrNotExist
	}
	return runID, run, nil
}

func getRunJobsByRunID(ctx *context.APIContext, runID int64) ([]*actions_model.ActionRunJob, error) {
	run, has, err := db.GetByID[actions_model.ActionRun](ctx, runID)
	if err != nil {
		return nil, err
	}
	if !has || run.RepoID != ctx.Repo.Repository.ID {
		return nil, util.ErrNotExist
	}
	run.Repo = ctx.Repo.Repository
	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	for _, v := range jobs {
		v.Run = run
	}
	return jobs, nil
}

func getRunJobsAndCurrent(ctx *context.APIContext, runID, jobIndex int64) (*actions_model.ActionRunJob, []*actions_model.ActionRunJob, error) {
	jobs, err := getRunJobsByRunID(ctx, runID)
	if err != nil {
		return nil, nil, err
	}
	if len(jobs) == 0 {
		return nil, nil, util.ErrNotExist
	}

	if jobIndex >= 0 && jobIndex < int64(len(jobs)) {
		return jobs[jobIndex], jobs, nil
	}
	return jobs[0], jobs, nil
}

// LogCursor represents a log cursor position
type LogCursor struct {
	Step     int   `json:"step"`
	Cursor   int64 `json:"cursor"`
	Expanded bool  `json:"expanded"`
}

// LogRequest represents a log streaming request
type LogRequest struct {
	LogCursors []LogCursor `json:"logCursors"`
}

// LogStepLine represents a single log line
type LogStepLine struct {
	Index     int64   `json:"index"`
	Message   string  `json:"message"`
	Timestamp float64 `json:"timestamp"`
}

// LogStep represents logs for a workflow step
type LogStep struct {
	Step    int            `json:"step"`
	Cursor  int64          `json:"cursor"`
	Lines   []*LogStepLine `json:"lines"`
	Started int64          `json:"started"`
}

// LogResponse represents the complete log response
type LogResponse struct {
	StepsLog []*LogStep `json:"stepsLog"`
}

func GetWorkflowRunLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/logs repository getWorkflowRunLogs
	// ---
	// summary: Download workflow run logs as archive
	// produces:
	// - application/zip
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: Logs archive
	//   "404":
	//     "$ref": "#/responses/notFound"

	_, run, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err = common.DownloadActionsRunAllJobLogs(ctx.Base, ctx.Repo.Repository, run.ID); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Logs not found")
		} else {
			ctx.APIErrorInternal(err)
		}
	}
}

func GetWorkflowJobLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/jobs/{job_id}/logs repository getWorkflowJobLogs
	// ---
	// summary: Download job logs
	// produces:
	// - application/zip
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// - name: job_id
	//   in: path
	//   description: id of the job
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: Job logs
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID, _, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	jobID := ctx.PathParamInt64("job_id")

	// Get the job by ID and verify it belongs to the run
	job, err := actions_model.GetRunJobByID(ctx, jobID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if job.RunID != runID {
		ctx.APIError(404, "Job not found in this run")
		return
	}

	if err = job.LoadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err = common.DownloadActionsRunJobLogs(ctx.Base, ctx.Repo.Repository, job); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Job logs not found")
		} else {
			ctx.APIErrorInternal(err)
		}
	}
}

func GetWorkflowRunLogsStream(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/logs repository getWorkflowRunLogsStream
	// ---
	// summary: Get streaming workflow run logs with cursor support
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// - name: job
	//   in: query
	//   description: job index (0-based), defaults to first job
	//   type: integer
	//   required: false
	// - name: body
	//   in: body
	//   schema:
	//     type: object
	//     properties:
	//       logCursors:
	//         type: array
	//         items:
	//           type: object
	//           properties:
	//             step:
	//               type: integer
	//             cursor:
	//               type: integer
	//             expanded:
	//               type: boolean
	// responses:
	//   "200":
	//     description: Streaming logs
	//     schema:
	//       type: object
	//       properties:
	//         stepsLog:
	//           type: array
	//           items:
	//             type: object
	//             properties:
	//               step:
	//                 type: integer
	//               cursor:
	//                 type: integer
	//               lines:
	//                 type: array
	//                 items:
	//                   type: object
	//                   properties:
	//                     index:
	//                       type: integer
	//                     message:
	//                       type: string
	//                     timestamp:
	//                       type: number
	//               started:
	//                 type: integer
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID, _, err := getRunID(ctx)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	jobID := ctx.FormInt64("job_id")
	jobIndex := int64(0)
	if jobID > 0 {
		jobs, err := getRunJobsByRunID(ctx, runID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		for i, j := range jobs {
			if j.ID == jobID {
				jobIndex = int64(i)
				break
			}
		}
		if jobIndex == 0 && jobID > 0 {
			ctx.APIError(404, "Job not found")
			return
		}
	}

	// Parse log cursors from request body
	var req LogRequest
	if err := json.NewDecoder(ctx.Req.Body).Decode(&req); err != nil {
		// If no body or invalid JSON, start with empty cursors
		req = LogRequest{LogCursors: []LogCursor{}}
	}

	current, _, err := getRunJobsAndCurrent(ctx, runID, jobIndex)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(404, "Run or job not found")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	var task *actions_model.ActionTask
	if current.TaskID > 0 {
		task, err = actions_model.GetTaskByID(ctx, current.TaskID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		task.Job = current
		if err := task.LoadAttributes(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	response := &LogResponse{
		StepsLog: make([]*LogStep, 0),
	}

	if task != nil {
		logs, err := convertToLogResponse(ctx, req.LogCursors, task)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		response.StepsLog = append(response.StepsLog, logs...)
	}

	ctx.JSON(http.StatusOK, response)
}

func convertToLogResponse(ctx *context.APIContext, cursors []LogCursor, task *actions_model.ActionTask) ([]*LogStep, error) {
	var logs []*LogStep
	steps := actions.FullSteps(task)

	for _, cursor := range cursors {
		if !cursor.Expanded {
			continue
		}

		if cursor.Step >= len(steps) {
			continue
		}

		step := steps[cursor.Step]

		// if task log is expired, return a consistent log line
		if task.LogExpired {
			if cursor.Cursor == 0 {
				logs = append(logs, &LogStep{
					Step:   cursor.Step,
					Cursor: 1,
					Lines: []*LogStepLine{
						{
							Index:     1,
							Message:   "Log has expired and is no longer available",
							Timestamp: float64(task.Updated.AsTime().UnixNano()) / float64(time.Second),
						},
					},
					Started: int64(step.Started),
				})
			}
			continue
		}

		logLines := make([]*LogStepLine, 0)

		index := step.LogIndex + cursor.Cursor
		validCursor := cursor.Cursor >= 0 &&
			cursor.Cursor < step.LogLength &&
			index < int64(len(task.LogIndexes))

		if validCursor {
			length := step.LogLength - cursor.Cursor
			offset := task.LogIndexes[index]
			logRows, err := actions.ReadLogs(ctx, task.LogInStorage, task.LogFilename, offset, length)
			if err != nil {
				return nil, err
			}

			for i, row := range logRows {
				logLines = append(logLines, &LogStepLine{
					Index:     cursor.Cursor + int64(i) + 1, // start at 1
					Message:   row.Content,
					Timestamp: float64(row.Time.AsTime().UnixNano()) / float64(time.Second),
				})
			}
		}

		logs = append(logs, &LogStep{
			Step:    cursor.Step,
			Cursor:  cursor.Cursor + int64(len(logLines)),
			Lines:   logLines,
			Started: int64(step.Started),
		})
	}

	return logs, nil
}
