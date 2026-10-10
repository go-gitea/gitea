// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	go_context "context"
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/httplib"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// GetArtifactsOfRun Lists all artifacts for a repository.
func GetArtifactsOfRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/artifacts repository getArtifactsOfRun
	// ---
	// summary: Lists all artifacts for a repository run
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
	//   description: runid of the workflow run
	//   type: integer
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the artifact
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ArtifactsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	artifactName := ctx.Req.URL.Query().Get("name")
	repoID, runID := ctx.Repo.Repository.ID, ctx.PathParamInt64("run")

	artifacts, total, err := db.FindAndCount[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RepoID:               repoID,
		RunID:                runID,
		ArtifactName:         artifactName,
		FinalizedArtifactsV4: true,
		ListOptions:          utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionArtifactsResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionArtifact, len(artifacts))
	for i := range artifacts {
		convertedArtifact, err := convert.ToActionArtifact(ctx.Repo.Repository, artifacts[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedArtifact
	}

	ctx.JSON(http.StatusOK, &res)
}

// GetArtifacts Lists all artifacts for a repository.
func GetArtifacts(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts repository getArtifacts
	// ---
	// summary: Lists all artifacts for a repository
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
	// - name: name
	//   in: query
	//   description: name of the artifact
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ArtifactsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoID := ctx.Repo.Repository.ID
	artifactName := ctx.Req.URL.Query().Get("name")

	artifacts, total, err := db.FindAndCount[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RepoID:               repoID,
		ArtifactName:         artifactName,
		FinalizedArtifactsV4: true,
		ListOptions:          utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionArtifactsResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionArtifact, len(artifacts))
	for i := range artifacts {
		convertedArtifact, err := convert.ToActionArtifact(ctx.Repo.Repository, artifacts[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedArtifact
	}

	ctx.JSON(http.StatusOK, &res)
}

// GetArtifact Gets a specific artifact for a workflow run.
func GetArtifact(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id} repository getArtifact
	// ---
	// summary: Gets a specific artifact for a workflow run
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
	// - name: artifact_id
	//   in: path
	//   description: id of the artifact
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Artifact"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	art := getArtifactByPathParam(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	if actions_service.IsArtifactV4(art) {
		convertedArtifact, err := convert.ToActionArtifact(ctx.Repo.Repository, art)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		ctx.JSON(http.StatusOK, convertedArtifact)
		return
	}
	// v3 not supported due to not having one unique id
	ctx.APIError(http.StatusNotFound, "Artifact not found")
}

// DeleteArtifact Deletes a specific artifact for a workflow run.
func DeleteArtifact(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/artifacts/{artifact_id} repository deleteArtifact
	// ---
	// summary: Deletes a specific artifact for a workflow run
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
	// - name: artifact_id
	//   in: path
	//   description: id of the artifact
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: "No Content"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	art := getArtifactByPathParam(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	if actions_service.IsArtifactV4(art) {
		if err := actions_model.SetArtifactNeedDeleteByID(ctx, art.ID); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		ctx.Status(http.StatusNoContent)
		return
	}
	// v3 not supported due to not having one unique id
	ctx.APIError(http.StatusNotFound, "Artifact not found")
}

func buildSignature(endp string, expires, artifactID int64) []byte {
	return actions.BuildSignature("api", endp, strconv.FormatInt(expires, 10), strconv.FormatInt(artifactID, 10))
}

func buildDownloadRawEndpoint(ownerName, repoName string, artifactID int64) string {
	return fmt.Sprintf("api/v1/repos/%s/%s/actions/artifacts/%d/zip/raw", url.PathEscape(ownerName), url.PathEscape(repoName), artifactID)
}

func buildSigURL(ctx go_context.Context, endPoint string, artifactID int64) string {
	// endPoint is a path like "api/v1/repos/owner/repo/actions/artifacts/1/zip/raw"
	expires := time.Now().Add(60 * time.Minute).Unix()
	uploadURL := httplib.GuessCurrentAppURL(ctx) + endPoint + "?sig=" + base64.RawURLEncoding.EncodeToString(buildSignature(endPoint, expires, artifactID)) + "&expires=" + strconv.FormatInt(expires, 10)
	return uploadURL
}

// DownloadArtifact Downloads a specific artifact for a workflow run redirects to blob url.
func DownloadArtifact(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip repository downloadArtifact
	// ---
	// summary: Downloads a specific artifact for a workflow run redirects to blob url
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
	// - name: artifact_id
	//   in: path
	//   description: id of the artifact
	//   type: string
	//   required: true
	// responses:
	//   "302":
	//     description: redirect to the blob download
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	art := getArtifactByPathParam(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	// if artifacts status is not uploaded-confirmed, treat it as not found
	if art.Status == actions_model.ArtifactStatusExpired {
		ctx.APIError(http.StatusNotFound, "Artifact has expired")
		return
	}

	if actions_service.IsArtifactV4(art) {
		// @actions/toolkit asserts that downloaded artifacts of a different runid return 302
		// https://github.com/actions/toolkit/blob/44d43b5490b02998bd09b0c4ff369a4cc67876c2/packages/artifact/src/internal/download/download-artifact.ts#L203-L210
		if actions_service.DownloadArtifactV4ServeDirect(ctx.Base, art) {
			return
		}

		// @actions/toolkit asserts a 302 for the artifact download, so we have to build a signed URL and redirect to it
		// TODO: a perma link to the code for reference
		redirectURL := buildSigURL(ctx, buildDownloadRawEndpoint(ctx.Repo.Repository.OwnerName, ctx.Repo.Repository.Name, art.ID), art.ID)
		ctx.Redirect(redirectURL, http.StatusFound)
		return
	}
	// v3 not supported due to not having one unique id
	ctx.APIError(http.StatusNotFound, "Artifact not found")
}

// DownloadArtifactRaw Downloads a specific artifact for a workflow run directly.
func DownloadArtifactRaw(ctx *context.APIContext) {
	// it doesn't use repoAssignment middleware, so it needs to prepare the repo and check permission (sig) by itself
	ownerName, repoName := ctx.PathParam("username"), ctx.PathParam("reponame")
	query := ctx.Req.URL.Query()
	sigBytes, _ := base64.RawURLEncoding.DecodeString(query.Get("sig"))
	expires, _ := strconv.ParseInt(query.Get("expires"), 10, 64)
	artifactID := ctx.PathParamInt64("artifact_id")

	if !hmac.Equal(sigBytes, buildSignature(buildDownloadRawEndpoint(ownerName, repoName, artifactID), expires, artifactID)) {
		ctx.APIErrorNotFound()
		return
	}
	if time.Unix(expires, 0).Before(time.Now()) {
		ctx.APIError(http.StatusUnauthorized, "Error link expired")
		return
	}

	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	art := getArtifactByPathParam(ctx, repo)
	if ctx.Written() {
		return
	}

	// if artifacts status is not uploaded-confirmed, treat it as not found
	if art.Status == actions_model.ArtifactStatusExpired {
		ctx.APIError(http.StatusNotFound, "Artifact has expired")
		return
	}
	if actions_service.IsArtifactV4(art) {
		err := actions_service.DownloadArtifactV4(ctx.Base, art)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		return
	}
	// v3 not supported due to not having one unique id
	ctx.APIError(http.StatusNotFound, "artifact not found")
}

// Try to get the artifact by ID and check access
func getArtifactByPathParam(ctx *context.APIContext, repo *repo_model.Repository) *actions_model.ActionArtifact {
	artifactID := ctx.PathParamInt64("artifact_id")

	art, ok, err := db.GetByID[actions_model.ActionArtifact](ctx, artifactID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	// if artifacts status is not uploaded-confirmed, treat it as not found
	// only check RepoID here, because the repository owner may change over the time
	if !ok ||
		art.RepoID != repo.ID ||
		art.Status != actions_model.ArtifactStatusUploadConfirmed && art.Status != actions_model.ArtifactStatusExpired {
		ctx.APIError(http.StatusNotFound, "artifact not found")
		return nil
	}
	return art
}
