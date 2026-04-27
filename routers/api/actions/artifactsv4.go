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
// 1.4. BlockList xml payload to Blobstorage (unauthenticated request)
// Files of about 800MB are parallel in parallel and / or out of order, this file is needed to ensure the correct order
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=blockList
// Request
// <?xml version="1.0" encoding="UTF-8" standalone="yes"?>
// <BlockList>
// 	<Latest>blockId1</Latest>
// 	<Latest>blockId2</Latest>
// </BlockList>
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
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"xorm.io/builder"
)

const ArtifactV4RouteBase = "/twirp/github.actions.results.api.v1.ArtifactService"

type artifactV4Routes struct {
	prefix string
	fs     storage.ObjectStorage
}

func ArtifactV4Contexter() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base := context.NewBaseContext(resp, req)
			ctx := &ArtifactContext{Base: base}
			ctx.SetContextValue(artifactContextKey, ctx)
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

func ArtifactsV4Routes(prefix string) *web.Router {
	m := web.NewRouter()

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

func (r *artifactV4Routes) buildSignature(endpoint, expires, artifactName string, taskID, artifactID int64) []byte {
	mac := hmac.New(sha256.New, setting.GetGeneralTokenSigningSecret())
	mac.Write([]byte(endpoint))
	mac.Write([]byte(expires))
	mac.Write([]byte(artifactName))
	_, _ = fmt.Fprint(mac, taskID)
	_, _ = fmt.Fprint(mac, artifactID)
	return mac.Sum(nil)
}

func (r *artifactV4Routes) buildArtifactURL(ctx *ArtifactContext, endpoint, artifactName string, taskID, artifactID int64) string {
	expires := time.Now().Add(60 * time.Minute).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	uploadURL := strings.TrimSuffix(httplib.GuessCurrentAppURL(ctx), "/") + strings.TrimSuffix(r.prefix, "/") +
		"/" + endpoint +
		"?sig=" + base64.RawURLEncoding.EncodeToString(r.buildSignature(endpoint, expires, artifactName, taskID, artifactID)) +
		"&expires=" + url.QueryEscape(expires) +
		"&artifactName=" + url.QueryEscape(artifactName) +
		"&taskID=" + strconv.FormatInt(taskID, 10) +
		"&artifactID=" + strconv.FormatInt(artifactID, 10)
	return uploadURL
}

func makeBlockFilenameV4(runID, artifactID, size int64, blockID string) string {
	sizeInName := max(size, 0) // do not use "-1" in filename
	return fmt.Sprintf("block-%d-%d-%d-%s", runID, artifactID, sizeInName, base64.URLEncoding.EncodeToString([]byte(blockID)))
}

var errSkipChunkFile = errors.New("skip this chunk file")

func parseChunkFileItemV4(st storage.ObjectStorage, artifactID int64, fpath string) (*chunkFileItem, error) {
	baseName := path.Base(fpath)
	if !strings.HasPrefix(baseName, "block-") {
		return nil, errSkipChunkFile
	}
	var item chunkFileItem
	var unusedRunID int64
	var b64chunkName string
	_, err := fmt.Sscanf(baseName, "block-%d-%d-%d-%s", &unusedRunID, &item.ArtifactID, &item.Size, &b64chunkName)
	if err != nil {
		return nil, err
	}
	if item.ArtifactID != artifactID {
		return nil, errSkipChunkFile
	}
	chunkName, err := base64.URLEncoding.DecodeString(b64chunkName)
	if err != nil {
		return nil, err
	}
	item.ChunkName = string(chunkName)
	item.Path = fpath
	if item.Size <= 0 {
		fi, err := st.Stat(item.Path)
		if err != nil {
			return nil, err
		}
		item.Size = fi.Size()
	}
	return &item, nil
}

func (r *artifactV4Routes) verifySignature(ctx *ArtifactContext, endp string) (*actions_model.ActionTask, string, bool) {
	rawTaskID := ctx.Req.URL.Query().Get("taskID")
	rawArtifactID := ctx.Req.URL.Query().Get("artifactID")
	sig := ctx.Req.URL.Query().Get("sig")
	expires := ctx.Req.URL.Query().Get("expires")
	artifactName := ctx.Req.URL.Query().Get("artifactName")
	dsig, errSig := base64.RawURLEncoding.DecodeString(sig)
	taskID, errTask := strconv.ParseInt(rawTaskID, 10, 64)
	artifactID, errArtifactID := strconv.ParseInt(rawArtifactID, 10, 64)
	err := errors.Join(errSig, errTask, errArtifactID)
	if err != nil {
		log.Error("Error decoding signature values: %v", err)
		ctx.HTTPError(http.StatusBadRequest, "Error decoding signature values")
		return nil, "", false
	}
	expecedsig := r.buildSignature(endp, expires, artifactName, taskID, artifactID)
	if !hmac.Equal(dsig, expecedsig) {
		log.Error("Error unauthorized")
		ctx.HTTPError(http.StatusUnauthorized, "Error unauthorized")
		return nil, "", false
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", expires)
	if err != nil || t.Before(time.Now()) {
		log.Error("Error link expired")
		ctx.HTTPError(http.StatusUnauthorized, "Error link expired")
		return nil, "", false
	}
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("Error runner api getting task by ID: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error runner api getting task by ID")
		return nil, "", false
	}
	if task.Status != actions_model.StatusRunning {
		log.Error("Error runner api getting task: task is not running")
		ctx.HTTPError(http.StatusInternalServerError, "Error runner api getting task: task is not running")
		return nil, "", false
	}
	if err := task.LoadJob(ctx); err != nil {
		log.Error("Error runner api getting job: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error runner api getting job")
		return nil, "", false
	}
	return task, artifactName, true
}

func (r *artifactV4Routes) getArtifactByName(ctx *ArtifactContext, runID, runAttemptID int64, name string) (*actions_model.ActionArtifact, error) {
	var art actions_model.ActionArtifact
	has, err := db.GetEngine(ctx).Where(builder.Eq{"run_id": runID, "run_attempt_id": runAttemptID, "artifact_name": name}, builder.Like{"content_encoding", "%/%"}).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}
	return &art, nil
}

func (r *artifactV4Routes) parseProtobufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) bool {
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		log.Error("Error decode request body: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error decode request body")
		return false
	}
	err = protojson.Unmarshal(body, req)
	if err != nil {
		log.Error("Error decode request body: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error decode request body")
		return false
	}
	return true
}

func (r *artifactV4Routes) sendProtobufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) {
	resp, err := protojson.Marshal(req)
	if err != nil {
		log.Error("Error encode response body: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error encode response body")
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/json;charset=utf-8")
	ctx.Resp.WriteHeader(http.StatusOK)
	_, _ = ctx.Resp.Write(resp)
}

func (r *artifactV4Routes) createArtifact(ctx *ArtifactContext) {
	var req CreateArtifactRequest

	if ok := r.parseProtobufBody(ctx, &req); !ok {
		return
	}
	_, _, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name

	retentionDays := setting.Actions.ArtifactRetentionDays
	if req.ExpiresAt != nil {
		retentionDays = int64(time.Until(req.ExpiresAt.AsTime()).Hours() / 24)
	}
	encoding := req.GetMimeType().GetValue()
	// Validate media type
	if encoding != "" {
		encoding, _, _ = mime.ParseMediaType(encoding)
	}
	fileName := artifactName
	if !strings.Contains(encoding, "/") || strings.EqualFold(encoding, actions_model.ContentTypeZip) && !strings.HasSuffix(fileName, ".zip") {
		encoding = actions_model.ContentTypeZip
		fileName = artifactName + ".zip"
	}
	// create or get artifact with name and path
	artifact, err := actions_model.CreateArtifact(ctx, ctx.ActionTask, artifactName, fileName, retentionDays)
	if err != nil {
		log.Error("Error create or get artifact: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error create or get artifact")
		return
	}
	artifact.ContentEncodingOrType = encoding
	artifact.FileSize = 0
	artifact.FileCompressedSize = 0

	var respData CreateArtifactResponse

	if setting.Actions.ArtifactStorage.ServeDirect() && setting.Actions.ArtifactStorage.Type == setting.AzureBlobStorageType {
		storagePath := generateArtifactStoragePath(artifact)
		if artifact.StoragePath != "" {
			_ = storage.ActionsArtifacts.Delete(artifact.StoragePath)
		}
		artifact.StoragePath = storagePath
		artifact.Status = actions_model.ArtifactStatusUploadPending
		u, err := storage.ActionsArtifacts.ServeDirectURL(artifact.StoragePath, artifact.ArtifactPath, http.MethodPut, nil)
		if err != nil {
			log.Error("Error ServeDirectURL: %v", err)
			ctx.HTTPError(http.StatusInternalServerError, "Error ServeDirectURL")
			return
		}
		respData = CreateArtifactResponse{
			Ok:              true,
			SignedUploadUrl: u.String(),
		}
	} else {
		respData = CreateArtifactResponse{
			Ok:              true,
			SignedUploadUrl: r.buildArtifactURL(ctx, "UploadArtifact", artifactName, ctx.ActionTask.ID, artifact.ID),
		}
	}

	if err := actions_model.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
		log.Error("Error UpdateArtifactByID: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error UpdateArtifactByID")
		return
	}

	r.sendProtobufBody(ctx, &respData)
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
		artifact, err := r.getArtifactByName(ctx, task.Job.RunID, task.Job.RunAttemptID, artifactName)
		if err != nil {
			log.Error("Error artifact not found: %v", err)
			ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
			return
		}
		blockID := ctx.Req.URL.Query().Get("blockid")
		if blockID == "" {
			uploadedLength, err := appendUploadChunkV3(r.fs, ctx, artifact, artifact.RunID, artifact.FileSize)
			if err != nil {
				log.Error("Error appending chunk %v", err)
				ctx.HTTPError(http.StatusInternalServerError, "Error appending Chunk")
				return
			}
			artifact.FileCompressedSize += uploadedLength
			artifact.FileSize += uploadedLength
			if err := actions_model.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
				log.Error("Error UpdateArtifactByID: %v", err)
				ctx.HTTPError(http.StatusInternalServerError, "Error UpdateArtifactByID")
				return
			}
		} else {
			blockFilename := makeBlockFilenameV4(task.Job.RunID, artifact.ID, ctx.Req.ContentLength, blockID)
			_, err := r.fs.Save(fmt.Sprintf("%s/%s", makeTmpPathNameV4(task.Job.RunID), blockFilename), ctx.Req.Body, ctx.Req.ContentLength)
			if err != nil {
				log.Error("Error uploading block blob %v", err)
				ctx.HTTPError(http.StatusInternalServerError, "Error uploading block blob")
				return
			}
		}
		ctx.JSON(http.StatusCreated, "appended")
	case "blocklist":
		rawArtifactID := ctx.Req.URL.Query().Get("artifactID")
		artifactID, _ := strconv.ParseInt(rawArtifactID, 10, 64)
		_, err := r.fs.Save(fmt.Sprintf("%s/%d-%d-blocklist", makeTmpPathNameV4(task.Job.RunID), task.Job.RunID, artifactID), ctx.Req.Body, -1)
		if err != nil {
			log.Error("Error uploading blocklist %v", err)
			ctx.HTTPError(http.StatusInternalServerError, "Error uploading blocklist")
			return
		}
		ctx.JSON(http.StatusCreated, "created")
	}
}

type BlockList struct {
	Latest []string `xml:"Latest"`
}

type Latest struct {
	Value string `xml:",chardata"`
}

func (r *artifactV4Routes) readBlockList(runID, artifactID int64) (*BlockList, error) {
	blockListName := fmt.Sprintf("%s/%d-%d-blocklist", makeTmpPathNameV4(runID), runID, artifactID)
	s, err := r.fs.Open(blockListName)
	if err != nil {
		return nil, err
	}

	xdec := xml.NewDecoder(s)
	blockList := &BlockList{}
	err = xdec.Decode(blockList)

	_ = s.Close()

	delerr := r.fs.Delete(blockListName)
	if delerr != nil {
		log.Warn("Failed to delete blockList %s: %v", blockListName, delerr)
	}
	if err != nil {
		return nil, err
	}
	return blockList, nil
}

func (r *artifactV4Routes) finalizeArtifact(ctx *ArtifactContext) {
	var req FinalizeArtifactRequest

	if ok := r.parseProtobufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, runID, ctx.ActionTask.Job.RunAttemptID, req.Name)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
		return
	}

	if setting.Actions.ArtifactStorage.ServeDirect() && setting.Actions.ArtifactStorage.Type == setting.AzureBlobStorageType {
		r.finalizeAzureServeDirect(ctx, &req, artifact)
	} else {
		r.finalizeDefaultArtifact(ctx, &req, artifact, runID)
	}

	// Return on finalize error
	if ctx.Written() {
		return
	}

	respData := FinalizeArtifactResponse{
		Ok:         true,
		ArtifactId: artifact.ID,
	}
	r.sendProtobufBody(ctx, &respData)
}

func (r *artifactV4Routes) finalizeDefaultArtifact(ctx *ArtifactContext, req *FinalizeArtifactRequest, artifact *actions_model.ActionArtifact, runID int64) {
	blockList, blockListErr := r.readBlockList(runID, artifact.ID)
	chunks, err := listOrderedChunksForArtifact(r.fs, runID, artifact.ID, blockList)
	if err != nil {
		log.Error("Error list chunks: %v", errors.Join(blockListErr, err))
		ctx.HTTPError(http.StatusInternalServerError, "Error list chunks")
		return
	}
	artifact.FileSize = chunks[len(chunks)-1].End + 1
	artifact.FileCompressedSize = chunks[len(chunks)-1].End + 1

	if req.Size != artifact.FileSize {
		log.Error("Error merge chunks size mismatch")
		ctx.HTTPError(http.StatusInternalServerError, "Error merge chunks size mismatch")
		return
	}

	if err := mergeChunksForArtifact(ctx, chunks, r.fs, artifact, req.GetHash().GetValue()); err != nil {
		log.Error("Error merge chunks: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error merge chunks")
		return
	}
}

func (r *artifactV4Routes) finalizeAzureServeDirect(ctx *ArtifactContext, req *FinalizeArtifactRequest, artifact *actions_model.ActionArtifact) {
	checksumValue, hasSha256Checksum := strings.CutPrefix(req.GetHash().GetValue(), "sha256:")
	var actualLength int64
	if hasSha256Checksum {
		hashSha256 := sha256.New()
		obj, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
		if err != nil {
			log.Error("Error read block: %v", err)
			ctx.HTTPError(http.StatusInternalServerError, "Error read block")
			return
		}
		defer obj.Close()
		actualLength, err = io.Copy(hashSha256, obj)
		if err != nil {
			log.Error("Error read block: %v", err)
			ctx.HTTPError(http.StatusInternalServerError, "Error read block")
			return
		}
		rawChecksum := hashSha256.Sum(nil)
		actualChecksum := hex.EncodeToString(rawChecksum)
		if checksumValue != actualChecksum {
			log.Error("Error merge chunks: checksum mismatch")
			ctx.HTTPError(http.StatusInternalServerError, "Error merge chunks: checksum mismatch")
			return
		}
	} else {
		fi, err := storage.ActionsArtifacts.Stat(artifact.StoragePath)
		if err != nil {
			log.Error("Error stat block: %v", err)
			ctx.HTTPError(http.StatusInternalServerError, "Error stat block")
			return
		}
		actualLength = fi.Size()
	}

	if req.Size != actualLength {
		log.Error("Error merge chunks: length mismatch")
		ctx.HTTPError(http.StatusInternalServerError, "Error merge chunks: length mismatch")
		return
	}

	// Update artifact metadata and status now that the upload is confirmed.
	artifact.FileSize = actualLength
	artifact.FileCompressedSize = actualLength
	artifact.Status = actions_model.ArtifactStatusUploadConfirmed
	if err := actions_model.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
		log.Error("Error UpdateArtifactByID: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error UpdateArtifactByID")
		return
	}
}

func (r *artifactV4Routes) listArtifacts(ctx *ArtifactContext) {
	var req ListArtifactsRequest

	if ok := r.parseProtobufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifacts, err := db.Find[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RunID:                runID,
		RunAttemptID:         optional.Some(ctx.ActionTask.Job.RunAttemptID),
		Status:               int(actions_model.ArtifactStatusUploadConfirmed),
		FinalizedArtifactsV4: true,
	})
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}

	list := []*ListArtifactsResponse_MonolithArtifact{}

	table := map[string]*ListArtifactsResponse_MonolithArtifact{}
	for _, artifact := range artifacts {
		if _, ok := table[artifact.ArtifactName]; ok || req.IdFilter != nil && artifact.ID != req.IdFilter.Value || req.NameFilter != nil && artifact.ArtifactName != req.NameFilter.Value {
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
	r.sendProtobufBody(ctx, &respData)
}

func (r *artifactV4Routes) getSignedArtifactURL(ctx *ArtifactContext) {
	var req GetSignedArtifactURLRequest

	if ok := r.parseProtobufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, runID, ctx.ActionTask.Job.RunAttemptID, artifactName)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
		return
	}
	if artifact.Status != actions_model.ArtifactStatusUploadConfirmed {
		log.Error("Error artifact not found: %s", artifact.Status.ToString())
		ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
		return
	}

	respData := GetSignedArtifactURLResponse{}

	if setting.Actions.ArtifactStorage.ServeDirect() {
		// DO NOT USE the http POST method coming from the getSignedArtifactURL endpoint
		u, err := actions.GetArtifactV4ServeDirectURL(artifact, http.MethodGet)
		if err == nil {
			respData.SignedUrl = u
		}
	}
	if respData.SignedUrl == "" {
		respData.SignedUrl = r.buildArtifactURL(ctx, "DownloadArtifact", artifactName, ctx.ActionTask.ID, artifact.ID)
	}
	r.sendProtobufBody(ctx, &respData)
}

func (r *artifactV4Routes) downloadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "DownloadArtifact")
	if !ok {
		return
	}

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, task.Job.RunID, task.Job.RunAttemptID, artifactName)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
		return
	}
	if artifact.Status != actions_model.ArtifactStatusUploadConfirmed {
		log.Error("Error artifact not found: %s", artifact.Status.ToString())
		ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
		return
	}

	err = actions.DownloadArtifactV4ReadStorage(ctx.Base, artifact)
	if err != nil {
		log.Error("Error serve artifact: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "failed to download artifact")
	}
}

func (r *artifactV4Routes) deleteArtifact(ctx *ArtifactContext) {
	var req DeleteArtifactRequest

	if ok := r.parseProtobufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	// get artifact by name
	artifact, err := r.getArtifactByName(ctx, runID, ctx.ActionTask.Job.RunAttemptID, req.Name)
	if err != nil {
		log.Error("Error artifact not found: %v", err)
		ctx.HTTPError(http.StatusNotFound, "Error artifact not found")
		return
	}

	err = actions_model.SetArtifactNeedDeleteByRunAttempt(ctx, runID, ctx.ActionTask.Job.RunAttemptID, req.Name)
	if err != nil {
		log.Error("Error deleting artifacts: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}

	respData := DeleteArtifactResponse{
		Ok:         true,
		ArtifactId: artifact.ID,
	}
	r.sendProtobufBody(ctx, &respData)
}
