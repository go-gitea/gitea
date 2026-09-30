// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"
	"strconv"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

const jobSummaryRouteBase = "/_apis/pipelines/workflows/{run_id}/jobs/{job_id}/steps/{step_index}/summary"

func uploadJobSummary(ctx *ArtifactContext) {
	task, _, ok := validateRunID(ctx)
	if !ok {
		return
	}

	jobID := ctx.PathParamInt64("job_id")
	if jobID <= 0 || task.Job.ID != jobID {
		ctx.HTTPError(http.StatusBadRequest, "job_id mismatch")
		return
	}

	stepIndex, err := strconv.ParseInt(ctx.PathParam("step_index"), 10, 64)
	if err != nil || stepIndex < 0 {
		ctx.HTTPError(http.StatusBadRequest, "invalid step_index")
		return
	}
	steps, err := actions_model.GetTaskStepsByTaskID(ctx, task.ID)
	if err != nil {
		log.Error("Error getting task steps: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error getting task steps")
		return
	}
	if !slices.ContainsFunc(steps, func(s *actions_model.ActionTaskStep) bool { return s.Index == stepIndex }) {
		ctx.HTTPError(http.StatusBadRequest, "step_index mismatch")
		return
	}

	contentType, ok := normalizeJobSummaryContentType(ctx.Req.Header.Get("Content-Type"))
	if !ok {
		ctx.HTTPError(http.StatusBadRequest, "invalid summary content type")
		return
	}

	body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, actions_model.MaxJobSummarySize+1))
	if err != nil {
		log.Error("Error reading job summary request body: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "read request body")
		return
	}
	message := "success"
	if len(body) == 0 {
		// PUT with an empty body clears any previously-stored summary for this step.
		if err := actions_model.DeleteActionRunJobSummary(ctx, task.Job.RepoID, task.Job.RunID, task.Job.RunAttemptID, task.Job.ID, stepIndex); err != nil {
			log.Error("Error deleting job summary: %v", err)
			ctx.HTTPError(http.StatusInternalServerError, "Error deleting job summary")
			return
		}
		message = "cleared"
	} else if err := actions_model.UpsertActionRunJobSummary(ctx, task.Job.RepoID, task.Job.RunID, task.Job.RunAttemptID, task.Job.ID, stepIndex, contentType, body); err != nil {
		if errors.Is(err, actions_model.ErrJobSummaryAggregateExceeded) {
			ctx.HTTPError(http.StatusBadRequest, "job summary aggregate size exceeded")
			return
		}
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.HTTPError(http.StatusBadRequest, "invalid summary")
			return
		}
		log.Error("Error upsert job summary: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error upsert job summary")
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"message":    message,
		"sizeBytes":  len(body),
		"runAttempt": task.Job.RunAttemptID,
	})
}

func normalizeJobSummaryContentType(contentType string) (string, bool) {
	if contentType == "" || contentType == "application/octet-stream" {
		return actions_model.JobSummaryContentTypeMarkdown, true
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", false
	}
	if mediaType != actions_model.JobSummaryContentTypeMarkdown {
		return "", false
	}
	return actions_model.JobSummaryContentTypeMarkdown, true
}
