// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	context_module "code.gitea.io/gitea/services/context"
)

// AnalysisRequest is the JSON body for upserting an attempt analysis.
type AnalysisRequest struct {
	Note   string  `json:"note"`
	TagIDs []int64 `json:"tag_ids"`
}

// analysisTagDTO is the serialized form of a failure tag attached to an analysis.
type analysisTagDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// analysisResponse is the JSON returned for GET/PUT.
type analysisResponse struct {
	Exists    bool             `json:"exists"`
	Note      string           `json:"note"`
	Tags      []analysisTagDTO `json:"tags"`
	CanEdit   bool             `json:"canEdit"`
	UpdatedAt int64            `json:"updatedAt,omitempty"`
}

// resolveAttempt resolves the attempt for an analysis request.
// It uses `?attempt=N` if provided (specific attempt number scoped to the run),
// otherwise falls back to the run's latest attempt.
// Writes 404 and returns nil on failure.
func resolveAttempt(ctx *context_module.Context) *actions_model.ActionRunAttempt {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return nil
	}
	var (
		attempt *actions_model.ActionRunAttempt
		err     error
	)
	if attemptNum := ctx.FormInt64("attempt"); attemptNum > 0 {
		attempt, err = actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, attemptNum)
	} else if run.LatestAttemptID > 0 {
		attempt, err = actions_model.GetRunAttemptByRepoAndID(ctx, ctx.Repo.Repository.ID, run.LatestAttemptID)
	} else {
		ctx.NotFound(nil)
		return nil
	}
	if errors.Is(err, util.ErrNotExist) {
		ctx.NotFound(nil)
		return nil
	} else if err != nil {
		ctx.ServerError("GetRunAttempt", err)
		return nil
	}
	if attempt.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return nil
	}
	return attempt
}

func canEditAnalysis(ctx *context_module.Context) bool {
	return ctx.IsSigned && ctx.Repo.Permission.CanWrite(unit.TypeActions)
}

func buildAnalysisResponse(ctx *context_module.Context, analysis *actions_model.ActionRunAnalysis) (*analysisResponse, error) {
	resp := &analysisResponse{
		Tags:    []analysisTagDTO{},
		CanEdit: canEditAnalysis(ctx),
	}
	if analysis == nil {
		return resp, nil
	}
	resp.Exists = true
	resp.Note = analysis.Note
	resp.UpdatedAt = int64(analysis.UpdatedUnix)
	tags, err := actions_model.GetAnalysisTags(ctx, analysis.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		resp.Tags = append(resp.Tags, analysisTagDTO{ID: t.ID, Name: t.Name, Color: t.Color})
	}
	return resp, nil
}

// GetAttemptAnalysis returns the analysis for the addressed attempt (or an empty payload if none).
func GetAttemptAnalysis(ctx *context_module.Context) {
	attempt := resolveAttempt(ctx)
	if ctx.Written() {
		return
	}
	analysis, err := actions_model.GetAnalysisByAttemptID(ctx, attempt.ID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.ServerError("GetAnalysisByAttemptID", err)
		return
	}
	resp, err := buildAnalysisResponse(ctx, analysis)
	if err != nil {
		ctx.ServerError("buildAnalysisResponse", err)
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// PutAttemptAnalysis creates or replaces the analysis on the addressed attempt.
func PutAttemptAnalysis(ctx *context_module.Context) {
	attempt := resolveAttempt(ctx)
	if ctx.Written() {
		return
	}
	req := web.GetForm(ctx).(*AnalysisRequest)
	analysis, err := actions_model.UpsertAnalysis(ctx, attempt.RepoID, attempt.RunID, attempt.ID, ctx.Doer.ID, req.Note, req.TagIDs)
	if err != nil {
		ctx.ServerError("UpsertAnalysis", err)
		return
	}
	resp, err := buildAnalysisResponse(ctx, analysis)
	if err != nil {
		ctx.ServerError("buildAnalysisResponse", err)
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// DeleteAttemptAnalysis removes the analysis (if any) on the addressed attempt.
func DeleteAttemptAnalysis(ctx *context_module.Context) {
	attempt := resolveAttempt(ctx)
	if ctx.Written() {
		return
	}
	if err := actions_model.DeleteAnalysis(ctx, attempt.RepoID, attempt.ID); err != nil {
		ctx.ServerError("DeleteAnalysis", err)
		return
	}
	ctx.JSONOK()
}

// failureTagDTO is the serialized form for the tag-management endpoints.
type failureTagDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// FailureTagRequest is the JSON body for create/update of a failure tag.
type FailureTagRequest struct {
	Name        string `json:"name" binding:"Required;MaxSize(50)"`
	Color       string `json:"color" binding:"MaxSize(7)"`
	Description string `json:"description" binding:"MaxSize(255)"`
}

// ListFailureTags returns all failure tags defined on the current repo.
func ListFailureTags(ctx *context_module.Context) {
	tags, err := actions_model.ListRepoFailureTags(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("ListRepoFailureTags", err)
		return
	}
	out := make([]failureTagDTO, 0, len(tags))
	for _, t := range tags {
		out = append(out, failureTagDTO{ID: t.ID, Name: t.Name, Color: t.Color, Description: t.Description})
	}
	ctx.JSON(http.StatusOK, out)
}

// CreateFailureTag adds a new failure tag to the current repo.
func CreateFailureTag(ctx *context_module.Context) {
	req := web.GetForm(ctx).(*FailureTagRequest)
	tag := &actions_model.ActionRunFailureTag{
		RepoID:      ctx.Repo.Repository.ID,
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	}
	if err := actions_model.CreateFailureTag(ctx, tag); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.HTTPError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.ServerError("CreateFailureTag", err)
		return
	}
	ctx.JSON(http.StatusOK, failureTagDTO{ID: tag.ID, Name: tag.Name, Color: tag.Color, Description: tag.Description})
}

// UpdateFailureTag mutates a failure tag on the current repo.
func UpdateFailureTag(ctx *context_module.Context) {
	tag, err := actions_model.GetFailureTagByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if errors.Is(err, util.ErrNotExist) {
		ctx.NotFound(nil)
		return
	} else if err != nil {
		ctx.ServerError("GetFailureTagByID", err)
		return
	}
	req := web.GetForm(ctx).(*FailureTagRequest)
	tag.Name = req.Name
	tag.Color = req.Color
	tag.Description = req.Description
	if err := actions_model.UpdateFailureTag(ctx, tag); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.HTTPError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.ServerError("UpdateFailureTag", err)
		return
	}
	ctx.JSON(http.StatusOK, failureTagDTO{ID: tag.ID, Name: tag.Name, Color: tag.Color, Description: tag.Description})
}

// DeleteFailureTag removes a failure tag (and unlinks it from any analyses).
func DeleteFailureTag(ctx *context_module.Context) {
	if err := actions_model.DeleteFailureTag(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id")); err != nil {
		ctx.ServerError("DeleteFailureTag", err)
		return
	}
	ctx.JSONOK()
}
