// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
)

const jobSummaryRouteBase = "/_apis/pipelines/workflows/{run_id}/jobs/{job_id}/summary"

func JobSummaryRoutes(prefix string) *web.Router {
	m := web.NewRouter()
	m.AfterRouting(ArtifactContexter())

	m.Put(jobSummaryRouteBase, uploadJobSummary)
	return m
}

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
		ctx.HTTPError(http.StatusInternalServerError, "read request body")
		return
	}
	if len(body) == 0 {
		ctx.JSON(http.StatusOK, map[string]string{"message": "empty"})
		return
	}

	contentType := ctx.Req.Header.Get("Content-Type")
	if contentType == "" || strings.HasPrefix(contentType, "application/octet-stream") {
		contentType = "text/markdown"
	} else {
		// Strip charset to keep storage normalized; we only store UTF-8 text content.
		if i := strings.Index(contentType, ";"); i > 0 {
			contentType = strings.TrimSpace(contentType[:i])
		}
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
