// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const jobSummaryRouteBase = "/_apis/pipelines/workflows/{run_id}/jobs/{job_id}/summary"

func uploadJobSummary(ctx *ArtifactContext) {
	task, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}

	jobID := ctx.PathParamInt64("job_id")
	if jobID <= 0 {
		ctx.HTTPError(http.StatusBadRequest, "invalid job_id")
		return
	}

	if task == nil || task.Job == nil {
		ctx.HTTPError(http.StatusInternalServerError, "task/job not loaded")
		return
	}
	if task.Job.ID != jobID {
		ctx.HTTPError(http.StatusBadRequest, "job_id mismatch")
		return
	}
	if task.Job.RunID != runID {
		ctx.HTTPError(http.StatusBadRequest, "run_id mismatch")
		return
	}

	body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, actions_model.MaxJobSummarySize+1))
	if err != nil {
		log.Error("Error reading job summary request body: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "read request body")
		return
	}
	if len(body) == 0 {
		ctx.JSON(http.StatusOK, map[string]string{"message": "empty"})
		return
	}

	contentType, ok := normalizeJobSummaryContentType(ctx.Req.Header.Get("Content-Type"))
	if !ok {
		ctx.HTTPError(http.StatusBadRequest, "invalid summary content type")
		return
	}

	if err := actions_model.UpsertActionRunJobSummary(ctx, task.Job.RepoID, task.Job.RunID, task.Job.RunAttemptID, task.Job.ID, contentType, body); err != nil {
		if errorsIsInvalidArg(err) {
			ctx.HTTPError(http.StatusBadRequest, "invalid summary")
			return
		}
		log.Error("Error upsert job summary: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error upsert job summary")
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"message":    "success",
		"sizeBytes":  strconv.Itoa(len(body)),
		"runAttempt": strconv.FormatInt(task.Job.RunAttemptID, 10),
	})
}

func errorsIsInvalidArg(err error) bool {
	return errors.Is(err, util.ErrInvalidArgument)
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
