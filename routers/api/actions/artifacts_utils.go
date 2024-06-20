// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const (
	artifactXTfsFileLengthHeader     = "x-tfs-filelength"
	artifactXActionsResultsMD5Header = "x-actions-results-md5"
)

// The rules are from https://github.com/actions/toolkit/blob/main/packages/artifact/src/internal/path-and-artifact-name-validation.ts#L32
var invalidArtifactNameChars = strings.Join([]string{"\\", "/", "\"", ":", "<", ">", "|", "*", "?", "\r", "\n"}, "")

func validateArtifactName(ctx *ArtifactContext, artifactName string) bool {
	if strings.ContainsAny(artifactName, invalidArtifactNameChars) {
		log.Error("Error checking artifact name contains invalid character")
		ctx.Error(http.StatusBadRequest, "Error checking artifact name contains invalid character")
		return false
	}
	return true
}

func validateRunID(ctx *ArtifactContext) (*actions.ActionTask, int64, bool) {
	task := ctx.ActionTask
	runID := ctx.PathParamInt64("run_id")
	if task.Job.RunID != runID {
		log.Error("Error runID not match")
		ctx.Error(http.StatusBadRequest, "run-id does not match")
		return nil, 0, false
	}
	return task, runID, true
}

func validateRunIDV4(ctx *ArtifactContext, rawRunID string) (*actions.ActionTask, int64, bool) { //nolint:unparam
	task := ctx.ActionTask
	runID, err := strconv.ParseInt(rawRunID, 10, 64)
	if err != nil || task.Job.RunID != runID {
		log.Error("Error runID not match")
		ctx.Error(http.StatusBadRequest, "run-id does not match")
		return nil, 0, false
	}
	return task, runID, true
}

func validateArtifactHash(ctx *ArtifactContext, artifactName string) bool {
	paramHash := ctx.PathParam("artifact_hash")
	// use artifact name to create upload url
	artifactHash := fmt.Sprintf("%x", md5.Sum([]byte(artifactName)))
	if paramHash == artifactHash {
		return true
	}
	log.Error("Invalid artifact hash: %s", paramHash)
	ctx.Error(http.StatusBadRequest, "Invalid artifact hash")
	return false
}

func parseArtifactItemPath(ctx *ArtifactContext) (string, string, bool) {
	// itemPath is generated from upload-artifact action
	// it's formatted as {artifact_name}/{artfict_path_in_runner}
	// act_runner in host mode on Windows, itemPath is joined by Windows slash '\'
	itemPath := util.PathJoinRelX(ctx.Req.URL.Query().Get("itemPath"))
	artifactName := strings.Split(itemPath, "/")[0]
	artifactPath := strings.TrimPrefix(itemPath, artifactName+"/")
	if !validateArtifactHash(ctx, artifactName) {
		return "", "", false
	}
	if !validateArtifactName(ctx, artifactName) {
		return "", "", false
	}
	return artifactName, artifactPath, true
}

// getUploadFileSize returns the size of the file to be uploaded.
// The raw size is the size of the file as reported by the header X-TFS-FileLength.
func getUploadFileSize(ctx *ArtifactContext) (int64, int64) {
	contentLength := ctx.Req.ContentLength
	xTfsLength, _ := strconv.ParseInt(ctx.Req.Header.Get(artifactXTfsFileLengthHeader), 10, 64)
	if xTfsLength > 0 {
		return xTfsLength, contentLength
	}
	return contentLength, contentLength
}
