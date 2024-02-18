// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// GitHub Actions Artifacts V4 API Simple Description
//
// 1. Upload artifact
// 1.1. CreateArtifact
// Post: /twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact
// Request:
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name": "test",
//     "version": 4
// }
// Response:
// {
//     "ok": true,
//     "signedUploadUrl": "http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75"
// }
// 1.2. Upload Zip Content to Blobstorage (unauthenticated request)
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=block
// 1.3. Continue Upload Zip Content to Blobstorage (unauthenticated request), repeat until everything is uploaded
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=appendBlock
// 1.4. Unknown xml payload to Blobstorage (unauthenticated request), ignored for now
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=blockList
// 1.5. FinalizeArtifact
// Post: /twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact
// Request
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name": "test",
//     "size": "2097",
//     "hash": "sha256:b6325614d5649338b87215d9536b3c0477729b8638994c74cdefacb020a2cad4"
// }
// Response
// {
//     "ok": true,
//     "artifactId": "4"
// }
// 2. Download artifact
// 2.1. ListArtifacts and optionally filter by artifact exact name or id
// Post: /twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts
// Request
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name_filter": "test"
// }
// Response
// {
//     "artifacts": [
//         {
//             "workflowRunBackendId": "21",
//             "workflowJobRunBackendId": "49",
//             "databaseId": "4",
//             "name": "test",
//             "size": "2093",
//             "createdAt": "2024-01-23T00:13:28Z"
//         }
//     ]
// }
// 2.2. GetSignedArtifactURL get the URL to download the artifact zip file of a specific artifact
// Post: /twirp/github.actions.results.api.v1.ArtifactService/GetSignedArtifactURL
// Request
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name": "test"
// }
// Response
// {
//     "signedUrl": "http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/DownloadArtifact?sig=wHzFOwpF-6220-5CA0CIRmAX9VbiTC2Mji89UOqo1E8=&expires=2024-01-23+21%3A51%3A56.872846295+%2B0100+CET&artifactName=test&taskID=76"
// }
// 2.3. Download Zip from Blobstorage (unauthenticated request)
// GET: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/DownloadArtifact?sig=wHzFOwpF-6220-5CA0CIRmAX9VbiTC2Mji89UOqo1E8=&expires=2024-01-23+21%3A51%3A56.872846295+%2B0100+CET&artifactName=test&taskID=76

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"

	"google.golang.org/protobuf/encoding/protojson"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const ArtifactV4RouteBase = "/twirp/github.actions.results.api.v1.ArtifactService"

type artifactV4Routes struct {
	prefix string
	fs     storage.ObjectStorage
}

func ArtifactV4Contexter() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := context.NewBaseContext(resp, req)
			defer baseCleanUp()

			ctx := &ArtifactContext{Base: base}
			ctx.AppendContextValue(artifactContextKey, ctx)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

func ArtifactsV4Routes(prefix string) *web.Route {
	m := web.NewRoute()

	r := artifactV4Routes{
		prefix: prefix,
		fs:     storage.ActionsArtifacts,
	}

	m.Group("", func() {
		m.Post("CreateArtifact", r.createArtifact)
		m.Post("FinalizeArtifact", r.finalizeArtifact)
		m.Post("ListArtifacts", r.listArtifacts)
		m.Post("GetSignedArtifactURL", r.getSignedArtifactURL)
		m.Post("DeleteArtifact", r.deleteArtifact)
	}, ArtifactContexter())
	m.Group("", func() {
		m.Put("UploadArtifact", r.uploadArtifact)
		m.Get("DownloadArtifact", r.downloadArtifact)
	}, ArtifactV4Contexter())

	return m
}

func (r artifactV4Routes) buildSignature(endp, expires, artifactName string, taskID int64) []byte {
	mac := hmac.New(sha256.New, []byte(setting.SecretKey))
	mac.Write([]byte(endp))
	mac.Write([]byte(expires))
	mac.Write([]byte(artifactName))
	mac.Write([]byte(fmt.Sprint(taskID)))
	return mac.Sum(nil)
}

func (r artifactV4Routes) buildArtifactURL(endp, expires, artifactName string, taskID int64) string {
	uploadURL := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(r.prefix, "/") +
		"/" + endp + "?sig=" + base64.URLEncoding.EncodeToString(r.buildSignature(endp, expires, artifactName, taskID)) + "&expires=" + url.QueryEscape(expires) + "&artifactName=" + url.QueryEscape(artifactName) + "&taskID=" + fmt.Sprint(taskID)
	return uploadURL
}

func (r artifactV4Routes) verifySignature(ctx *ArtifactContext, endp string) (task *actions.ActionTask, artifactName string, ok bool) {
	rawTaskID := ctx.Req.URL.Query().Get("taskID")
	sig := ctx.Req.URL.Query().Get("sig")
	expires := ctx.Req.URL.Query().Get("expires")
	artifactName = ctx.Req.URL.Query().Get("artifactName")
	dsig, _ := base64.URLEncoding.DecodeString(sig)
	taskID, _ := strconv.ParseInt(rawTaskID, 10, 64)

	expecedsig := r.buildSignature(endp, expires, artifactName, taskID)
	if !hmac.Equal(dsig, expecedsig) {
		log.Error("Error unauthorized")
		ctx.Error(http.StatusUnauthorized, "Error unauthorized")
		return
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", expires)
	if err != nil || t.Before(time.Now()) {
		log.Error("Error link expired")
		ctx.Error(http.StatusUnauthorized, "Error link expired")
		return
	}
	task, err = actions.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("Error runner api getting task by ID: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error runner api getting task by ID")
		return
	}
	if task.Status != actions.StatusRunning {
		log.Error("Error runner api getting task: task is not running")
		ctx.Error(http.StatusInternalServerError, "Error runner api getting task: task is not running")
		return
	}
	if err := task.LoadJob(ctx); err != nil {
		log.Error("Error runner api getting job: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error runner api getting job")
		return
	}
	ok = true
	return
}

func (r *artifactV4Routes) getArtifactByName(ctx *ArtifactContext, runID int64, name string) (*actions.ActionArtifact, error) {
	var art actions.ActionArtifact
	has, err := db.GetEngine(ctx).Where("run_id = ? AND artifact_name = ? AND artifact_path = ?", runID, name, name+".zip").Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}
	return &art, nil
}

func (r *artifactV4Routes) parseProtbufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) bool {
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		log.Error("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return false
	}
	err = protojson.Unmarshal(body, req)
	if err != nil {
		log.Error("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return false
	}
	return true
}

func (r *artifactV4Routes) sendProtbufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) {
	resp, err := protojson.Marshal(req)
	if err != nil {
		log.Error("Error encode response body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error encode response body")
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/json;charset=utf-8")
	ctx.Resp.WriteHeader(http.StatusOK)
	_, _ = ctx.Resp.Write(resp)
}

func (r *artifactV4Routes) createArtifact(ctx *ArtifactContext) {
	var req CreateArtifactRequest

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, _, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name

	rententionDays := setting.Actions.ArtifactRetentionDays
	if req.ExpiresAt != nil {
		rententionDays = int64(time.Until(req.ExpiresAt.AsTime()).Hours() / 24)
	}
	// create or get artifact with name and path
	artifact, err := actions.CreateArtifact(ctx, ctx.ActionTask, artifactName, artifactName+".zip", rententionDays)
	if err != nil {
		log.Error("Error create or get artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error create or get artifact")
		return
	}
	artifact.ContentEncoding = "application/zip"
	if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
		log.Error("Error UpdateArtifactByID: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error UpdateArtifactByID")
		return
	}

	expires := time.Now().Add(60 * time.Minute).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	respData := CreateArtifactResponse{
		Ok:              true,
		SignedUploadUrl: r.buildArtifactURL("UploadArtifact", expires, artifactName, ctx.ActionTask.ID),
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) uploadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "UploadArtifact")
	if !ok {
		return
	}

	comp := ctx.Req.URL.Query().Get("comp")
	switch comp {
	case "block", "appendBlock":
		// get artifact by name
		artifact, err := r.getArtifactByName(ctx, task.Job.RunID, artifactName)
		if err != nil {
			log.Error("Error artifact not found: %v", err)
			ctx.Error(http.StatusNotFound, "Error artifact not found")
			return
		}

		if comp == "block" {
			artifact.FileSize = 0
			artifact.FileCompressedSize = 0
		}

		_, err = appendUploadChunk(r.fs, ctx, artifact, artifact.FileSize, ctx.Req.ContentLength, artifact.RunID)
		if err != nil {
			log.Error("Error runner api getting task: task is not running")
			ctx.Error(http.StatusInternalServerError, "Error runner api getting task: task is not running")
			return
		}
		artifact.FileCompressedSize += ctx.Req.ContentLength
		artifact.FileSize += ctx.Req.ContentLength
		if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
			log.Error("Error UpdateArtifactByID: %v", err)
			ctx.Error(http.StatusInternalServerError, "Error UpdateArtifactByID")
			return
		}
		ctx.JSON(http.StatusCreated, "appended")
	case "blocklist":
		ctx.JSON(http.StatusCreated, "created")
	}
}

func (r *artifactV4Routes) finalizeArtifact(ctx *ArtifactContext) {
	var req FinalizeArtifactRequest

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, runID, req.Name)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.Error(http.StatusNotFound, "Error artifact not found")
		return
	}
	chunkMap, err := listChunksByRunID(r.fs, runID)
	if err != nil {
		log.Error("Error merge chunks: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error merge chunks")
		return
	}
	chunks, ok := chunkMap[artifact.ID]
	if !ok {
		log.Error("Error merge chunks")
		ctx.Error(http.StatusInternalServerError, "Error merge chunks")
		return
	}
	checksum := ""
	if req.Hash != nil {
		checksum = req.Hash.Value
	}
	if err := mergeChunksForArtifact(ctx, chunks, r.fs, artifact, checksum); err != nil {
		log.Error("Error merge chunks: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error merge chunks")
		return
	}

	respData := FinalizeArtifactResponse{
		Ok:         true,
		ArtifactId: artifact.ID,
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) listArtifacts(ctx *ArtifactContext) {
	var req ListArtifactsRequest

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifacts, err := db.Find[actions.ActionArtifact](ctx, actions.FindArtifactsOptions{RunID: runID})
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if len(artifacts) == 0 {
		log.Debug("[artifact] handleListArtifacts, no artifacts")
		ctx.Error(http.StatusNotFound)
		return
	}

	list := []*ListArtifactsResponse_MonolithArtifact{}

	table := map[string]*ListArtifactsResponse_MonolithArtifact{}
	for _, artifact := range artifacts {
		if _, ok := table[artifact.ArtifactName]; ok || req.IdFilter != nil && artifact.ID != req.IdFilter.Value || req.NameFilter != nil && artifact.ArtifactName != req.NameFilter.Value || artifact.ArtifactName+".zip" != artifact.ArtifactPath || artifact.ContentEncoding != "application/zip" {
			table[artifact.ArtifactName] = nil
			continue
		}

		table[artifact.ArtifactName] = &ListArtifactsResponse_MonolithArtifact{
			Name:                    artifact.ArtifactName,
			CreatedAt:               timestamppb.New(artifact.CreatedUnix.AsTime()),
			DatabaseId:              artifact.ID,
			WorkflowRunBackendId:    req.WorkflowRunBackendId,
			WorkflowJobRunBackendId: req.WorkflowJobRunBackendId,
			Size:                    artifact.FileSize,
		}
	}
	for _, artifact := range table {
		if artifact != nil {
			list = append(list, artifact)
		}
	}

	respData := ListArtifactsResponse{
		Artifacts: list,
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) getSignedArtifactURL(ctx *ArtifactContext) {
	var req GetSignedArtifactURLRequest

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, runID, artifactName)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.Error(http.StatusNotFound, "Error artifact not found")
		return
	}

	respData := GetSignedArtifactURLResponse{}

	if setting.Actions.ArtifactStorage.MinioConfig.ServeDirect {
		u, err := storage.ActionsArtifacts.URL(artifact.StoragePath, artifact.ArtifactPath)
		if u != nil && err == nil {
			respData.SignedUrl = u.String()
		}
	}
	if respData.SignedUrl == "" {
		expires := time.Now().Add(60 * time.Minute).Format("2006-01-02 15:04:05.999999999 -0700 MST")
		respData.SignedUrl = r.buildArtifactURL("DownloadArtifact", expires, artifactName, ctx.ActionTask.ID)
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) downloadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "DownloadArtifact")
	if !ok {
		return
	}

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, task.Job.RunID, artifactName)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.Error(http.StatusNotFound, "Error artifact not found")
		return
	}

	file, _ := r.fs.Open(artifact.StoragePath)

	_, _ = io.Copy(ctx.Resp, file)
}

func (r *artifactV4Routes) deleteArtifact(ctx *ArtifactContext) {
	var req DeleteArtifactRequest

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, runID, req.Name)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.Error(http.StatusNotFound, "Error artifact not found")
		return
	}

	_, err = db.DeleteByID[actions.ActionArtifact](ctx, artifact.ID)
	err = errors.Join(err, r.fs.Delete(artifact.StoragePath))
	if err != nil {
		log.Error("Error deleting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	respData := DeleteArtifactResponse{
		Ok:         true,
		ArtifactId: artifact.ID,
	}
	r.sendProtbufBody(ctx, &respData)
}
