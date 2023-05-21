// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// GitHub Actions Artifacts API Simple Description
//
// 1. Upload artifact
// 1.1. Post upload url
// Post: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts?api-version=6.0-preview
// Request:
// {
//  "Type": "actions_storage",
//  "Name": "artifact"
// }
// Response:
// {
// 	"fileContainerResourceUrl":"/api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/upload"
// }
// it acquires an upload url for artifact upload
// 1.2. Upload artifact
// PUT: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/upload?itemPath=artifact%2Ffilename
// it upload chunk with headers:
//    x-tfs-filelength: 1024 					// total file length
//    content-length: 1024 						// chunk length
//    x-actions-results-md5: md5sum 	// md5sum of chunk
//    content-range: bytes 0-1023/1024 // chunk range
// we save all chunks to one storage directory after md5sum check
// 1.3. Confirm upload
// PATCH: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/upload?itemPath=artifact%2Ffilename
// it confirm upload and merge all chunks to one file, save this file to storage
//
// 2. Download artifact
// 2.1 list artifacts
// GET: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts?api-version=6.0-preview
// Response:
// {
// 	"count": 1,
// 	"value": [
// 		{
// 			"name": "artifact",
// 			"fileContainerResourceUrl": "/api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/path"
// 		}
// 	]
// }
// 2.2 download artifact
// GET: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/path?api-version=6.0-preview
// Response:
// {
//   "value": [
// 			{
// 	 			"contentLocation": "/api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/download",
// 				"path": "artifact/filename",
// 				"itemType": "file"
// 			}
//   ]
// }
// 2.3 download artifact file
// GET: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/download?itemPath=artifact%2Ffilename
// Response:
// download file
//

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
)

const (
	artifactXTfsFileLengthHeader     = "x-tfs-filelength"
	artifactXActionsResultsMD5Header = "x-actions-results-md5"
)

const artifactRouteBase = "/_apis/pipelines/workflows/{run_id}/artifacts"

type artifactContextKeyType struct{}

var artifactContextKey = artifactContextKeyType{}

type ArtifactContext struct {
	*context.Base

	ActionTask *actions.ActionTask
}

func init() {
	web.RegisterHandleTypeProvider[*ArtifactContext](func(req *http.Request) web.ResponseStatusProvider {
		return req.Context().Value(artifactContextKey).(*ArtifactContext)
	})
}

func ArtifactsRoutes(prefix string) *web.Route {
	m := web.NewRoute()
	m.Use(ArtifactContexter())

	r := artifactRoutes{
		prefix: prefix,
		fs:     storage.ActionsArtifacts,
	}

	m.Group(artifactRouteBase, func() {
		// retrieve, list and confirm artifacts
		m.Combo("").Get(r.listArtifacts).Post(r.getUploadArtifactURL).Patch(r.comfirmUploadArtifact)
		// handle container artifacts list and download
		m.Group("/{artifact_id}", func() {
			m.Put("/upload", r.uploadArtifact)
			m.Get("/path", r.getDownloadArtifactURL)
			m.Get("/download", r.downloadArtifact)
		})
	})

	return m
}

func ArtifactContexter() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := context.NewBaseContext(resp, req)
			defer baseCleanUp()

			ctx := &ArtifactContext{Base: base}
			ctx.AppendContextValue(artifactContextKey, ctx)

			// action task call server api with Bearer ACTIONS_RUNTIME_TOKEN
			// we should verify the ACTIONS_RUNTIME_TOKEN
			authHeader := req.Header.Get("Authorization")
			if len(authHeader) == 0 || !strings.HasPrefix(authHeader, "Bearer ") {
				ctx.Error(http.StatusUnauthorized, "Bad authorization header")
				return
			}

			authToken := strings.TrimPrefix(authHeader, "Bearer ")
			task, err := actions.GetRunningTaskByToken(req.Context(), authToken)
			if err != nil {
				log.Error("Error runner api getting task: %v", err)
				ctx.Error(http.StatusInternalServerError, "Error runner api getting task")
				return
			}

			if err := task.LoadJob(req.Context()); err != nil {
				log.Error("Error runner api getting job: %v", err)
				ctx.Error(http.StatusInternalServerError, "Error runner api getting job")
				return
			}

			ctx.ActionTask = task
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

type artifactRoutes struct {
	prefix string
	fs     storage.ObjectStorage
}

func (ar artifactRoutes) buildArtifactURL(runID, artifactID int64, suffix string) string {
	uploadURL := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ar.prefix, "/") +
		strings.ReplaceAll(artifactRouteBase, "{run_id}", strconv.FormatInt(runID, 10)) +
		"/" + strconv.FormatInt(artifactID, 10) + "/" + suffix
	return uploadURL
}

type getUploadArtifactRequest struct {
	Type string
	Name string
}

type getUploadArtifactResponse struct {
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

func (ar artifactRoutes) validateRunID(ctx *ArtifactContext) (*actions.ActionTask, int64, bool) {
	task := ctx.ActionTask
	runID := ctx.ParamsInt64("run_id")
	if task.Job.RunID != runID {
		log.Error("Error runID not match")
		ctx.Error(http.StatusBadRequest, "run-id does not match")
		return nil, 0, false
	}
	return task, runID, true
}

// getUploadArtifactURL generates a URL for uploading an artifact
func (ar artifactRoutes) getUploadArtifactURL(ctx *ArtifactContext) {
	task, runID, ok := ar.validateRunID(ctx)
	if !ok {
		return
	}

	var req getUploadArtifactRequest
	if err := json.NewDecoder(ctx.Req.Body).Decode(&req); err != nil {
		log.Error("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return
	}

	artifact, err := actions.CreateArtifact(ctx, task, req.Name)
	if err != nil {
		log.Error("Error creating artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	resp := getUploadArtifactResponse{
		FileContainerResourceURL: ar.buildArtifactURL(runID, artifact.ID, "upload"),
	}
	log.Debug("[artifact] get upload url: %s, artifact id: %d", resp.FileContainerResourceURL, artifact.ID)
	ctx.JSON(http.StatusOK, resp)
}

// getUploadFileSize returns the size of the file to be uploaded.
// The raw size is the size of the file as reported by the header X-TFS-FileLength.
func (ar artifactRoutes) getUploadFileSize(ctx *ArtifactContext) (int64, int64, error) {
	contentLength := ctx.Req.ContentLength
	xTfsLength, _ := strconv.ParseInt(ctx.Req.Header.Get(artifactXTfsFileLengthHeader), 10, 64)
	if xTfsLength > 0 {
		return xTfsLength, contentLength, nil
	}
	return contentLength, contentLength, nil
}

func (ar artifactRoutes) saveUploadChunk(ctx *ArtifactContext,
	artifact *actions.ActionArtifact,
	contentSize, runID int64,
) (int64, error) {
	contentRange := ctx.Req.Header.Get("Content-Range")
	start, end, length := int64(0), int64(0), int64(0)
	if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &length); err != nil {
		return -1, fmt.Errorf("parse content range error: %v", err)
	}

	storagePath := fmt.Sprintf("tmp%d/%d-%d-%d.chunk", runID, artifact.ID, start, end)

	// use io.TeeReader to avoid reading all body to md5 sum.
	// it writes data to hasher after reading end
	// if hash is not matched, delete the read-end result
	hasher := md5.New()
	r := io.TeeReader(ctx.Req.Body, hasher)

	// save chunk to storage
	writtenSize, err := ar.fs.Save(storagePath, r, -1)
	if err != nil {
		return -1, fmt.Errorf("save chunk to storage error: %v", err)
	}

	// check md5
	reqMd5String := ctx.Req.Header.Get(artifactXActionsResultsMD5Header)
	chunkMd5String := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	log.Debug("[artifact] check chunk md5, sum: %s, header: %s", chunkMd5String, reqMd5String)
	if reqMd5String != chunkMd5String || writtenSize != contentSize {
		if err := ar.fs.Delete(storagePath); err != nil {
			log.Error("Error deleting chunk: %s, %v", storagePath, err)
		}
		return -1, fmt.Errorf("md5 not match")
	}

	log.Debug("[artifact] save chunk %s, size: %d, artifact id: %d, start: %d, end: %d",
		storagePath, contentSize, artifact.ID, start, end)

	return length, nil
}

// The rules are from https://github.com/actions/toolkit/blob/main/packages/artifact/src/internal/path-and-artifact-name-validation.ts#L32
var invalidArtifactNameChars = strings.Join([]string{"\\", "/", "\"", ":", "<", ">", "|", "*", "?", "\r", "\n"}, "")

func (ar artifactRoutes) uploadArtifact(ctx *ArtifactContext) {
	_, runID, ok := ar.validateRunID(ctx)
	if !ok {
		return
	}
	artifactID := ctx.ParamsInt64("artifact_id")

	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if errors.Is(err, util.ErrNotExist) {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	// itemPath is generated from upload-artifact action
	// it's formatted as {artifact_name}/{artfict_path_in_runner}
	itemPath := util.PathJoinRel(ctx.Req.URL.Query().Get("itemPath"))
	artifactName := strings.Split(itemPath, "/")[0]

	// checkArtifactName checks if the artifact name contains invalid characters.
	// If the name contains invalid characters, an error is returned.
	if strings.ContainsAny(artifactName, invalidArtifactNameChars) {
		log.Error("Error checking artifact name contains invalid character")
		ctx.Error(http.StatusBadRequest, err.Error())
		return
	}

	// get upload file size
	fileSize, contentLength, err := ar.getUploadFileSize(ctx)
	if err != nil {
		log.Error("Error getting upload file size: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	// save chunk
	chunkAllLength, err := ar.saveUploadChunk(ctx, artifact, contentLength, runID)
	if err != nil {
		log.Error("Error saving upload chunk: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	// if artifact name is not set, update it
	if artifact.ArtifactName == "" {
		artifact.ArtifactName = artifactName
		artifact.ArtifactPath = itemPath // path in container
		artifact.FileSize = fileSize     // this is total size of all chunks
		artifact.FileCompressedSize = chunkAllLength
		artifact.ContentEncoding = ctx.Req.Header.Get("Content-Encoding")
		if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
			log.Error("Error updating artifact: %v", err)
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"message": "success",
	})
}

// comfirmUploadArtifact comfirm upload artifact.
// if all chunks are uploaded, merge them to one file.
func (ar artifactRoutes) comfirmUploadArtifact(ctx *ArtifactContext) {
	_, runID, ok := ar.validateRunID(ctx)
	if !ok {
		return
	}
	if err := ar.mergeArtifactChunks(ctx, runID); err != nil {
		log.Error("Error merging chunks: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"message": "success",
	})
}

type chunkItem struct {
	ArtifactID int64
	Start      int64
	End        int64
	Path       string
}

func (ar artifactRoutes) mergeArtifactChunks(ctx *ArtifactContext, runID int64) error {
	storageDir := fmt.Sprintf("tmp%d", runID)
	var chunks []*chunkItem
	if err := ar.fs.IterateObjects(storageDir, func(path string, obj storage.Object) error {
		item := chunkItem{Path: path}
		if _, err := fmt.Sscanf(path, storageDir+"/%d-%d-%d.chunk", &item.ArtifactID, &item.Start, &item.End); err != nil {
			return fmt.Errorf("parse content range error: %v", err)
		}
		chunks = append(chunks, &item)
		return nil
	}); err != nil {
		return err
	}
	// group chunks by artifact id
	chunksMap := make(map[int64][]*chunkItem)
	for _, c := range chunks {
		chunksMap[c.ArtifactID] = append(chunksMap[c.ArtifactID], c)
	}

	for artifactID, cs := range chunksMap {
		// get artifact to handle merged chunks
		artifact, err := actions.GetArtifactByID(ctx, cs[0].ArtifactID)
		if err != nil {
			return fmt.Errorf("get artifact error: %v", err)
		}

		sort.Slice(cs, func(i, j int) bool {
			return cs[i].Start < cs[j].Start
		})

		allChunks := make([]*chunkItem, 0)
		startAt := int64(-1)
		// check if all chunks are uploaded and in order and clean repeated chunks
		for _, c := range cs {
			// startAt is -1 means this is the first chunk
			// previous c.ChunkEnd + 1 == c.ChunkStart means this chunk is in order
			// StartAt is not -1 and c.ChunkStart is not startAt + 1 means there is a chunk missing
			if c.Start == (startAt + 1) {
				allChunks = append(allChunks, c)
				startAt = c.End
			}
		}

		// if the last chunk.End + 1 is not equal to chunk.ChunkLength, means chunks are not uploaded completely
		if startAt+1 != artifact.FileCompressedSize {
			log.Debug("[artifact] chunks are not uploaded completely, artifact_id: %d", artifactID)
			break
		}

		// use multiReader
		readers := make([]io.Reader, 0, len(allChunks))
		closeReaders := func() {
			for _, r := range readers {
				_ = r.(io.Closer).Close() // it guarantees to be io.Closer by the following loop's Open function
			}
			readers = nil
		}
		defer closeReaders()

		for _, c := range allChunks {
			var readCloser io.ReadCloser
			if readCloser, err = ar.fs.Open(c.Path); err != nil {
				return fmt.Errorf("open chunk error: %v, %s", err, c.Path)
			}
			readers = append(readers, readCloser)
		}
		mergedReader := io.MultiReader(readers...)

		// if chunk is gzip, decompress it
		if artifact.ContentEncoding == "gzip" {
			var err error
			mergedReader, err = gzip.NewReader(mergedReader)
			if err != nil {
				return fmt.Errorf("gzip reader error: %v", err)
			}
		}

		// save merged file
		storagePath := fmt.Sprintf("%d/%d/%d.chunk", runID%255, artifactID%255, time.Now().UnixNano())
		written, err := ar.fs.Save(storagePath, mergedReader, -1)
		if err != nil {
			return fmt.Errorf("save merged file error: %v", err)
		}
		if written != artifact.FileSize {
			return fmt.Errorf("merged file size is not equal to chunk length")
		}

		// save storage path to artifact
		log.Debug("[artifact] merge chunks to artifact: %d, %s", artifact.ID, storagePath)
		artifact.StoragePath = storagePath
		artifact.Status = actions.ArtifactStatusUploadConfirmed
		if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
			return fmt.Errorf("update artifact error: %v", err)
		}

		closeReaders() // close before delete

		// drop chunks
		for _, c := range cs {
			if err := ar.fs.Delete(c.Path); err != nil {
				return fmt.Errorf("delete chunk file error: %v", err)
			}
		}
	}
	return nil
}

type (
	listArtifactsResponse struct {
		Count int64                       `json:"count"`
		Value []listArtifactsResponseItem `json:"value"`
	}
	listArtifactsResponseItem struct {
		Name                     string `json:"name"`
		FileContainerResourceURL string `json:"fileContainerResourceUrl"`
	}
)

func (ar artifactRoutes) listArtifacts(ctx *ArtifactContext) {
	_, runID, ok := ar.validateRunID(ctx)
	if !ok {
		return
	}

	artifacts, err := actions.ListArtifactsByRunID(ctx, runID)
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	artficatsData := make([]listArtifactsResponseItem, 0, len(artifacts))
	for _, a := range artifacts {
		artficatsData = append(artficatsData, listArtifactsResponseItem{
			Name:                     a.ArtifactName,
			FileContainerResourceURL: ar.buildArtifactURL(runID, a.ID, "path"),
		})
	}
	respData := listArtifactsResponse{
		Count: int64(len(artficatsData)),
		Value: artficatsData,
	}
	ctx.JSON(http.StatusOK, respData)
}

type (
	downloadArtifactResponse struct {
		Value []downloadArtifactResponseItem `json:"value"`
	}
	downloadArtifactResponseItem struct {
		Path            string `json:"path"`
		ItemType        string `json:"itemType"`
		ContentLocation string `json:"contentLocation"`
	}
)

func (ar artifactRoutes) getDownloadArtifactURL(ctx *ArtifactContext) {
	_, runID, ok := ar.validateRunID(ctx)
	if !ok {
		return
	}

	artifactID := ctx.ParamsInt64("artifact_id")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if errors.Is(err, util.ErrNotExist) {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	downloadURL := ar.buildArtifactURL(runID, artifact.ID, "download")
	itemPath := util.PathJoinRel(ctx.Req.URL.Query().Get("itemPath"))
	respData := downloadArtifactResponse{
		Value: []downloadArtifactResponseItem{{
			Path:            util.PathJoinRel(itemPath, artifact.ArtifactPath),
			ItemType:        "file",
			ContentLocation: downloadURL,
		}},
	}
	ctx.JSON(http.StatusOK, respData)
}

func (ar artifactRoutes) downloadArtifact(ctx *ArtifactContext) {
	_, runID, ok := ar.validateRunID(ctx)
	if !ok {
		return
	}

	artifactID := ctx.ParamsInt64("artifact_id")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if errors.Is(err, util.ErrNotExist) {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if artifact.RunID != runID {
		log.Error("Error dismatch runID and artifactID, task: %v, artifact: %v", runID, artifactID)
		ctx.Error(http.StatusBadRequest, err.Error())
		return
	}

	fd, err := ar.fs.Open(artifact.StoragePath)
	if err != nil {
		log.Error("Error opening file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	defer fd.Close()

	if strings.HasSuffix(artifact.ArtifactPath, ".gz") {
		ctx.Resp.Header().Set("Content-Encoding", "gzip")
	}
	ctx.ServeContent(fd, &context.ServeHeaderOptions{
		Filename:     artifact.ArtifactName,
		LastModified: artifact.CreatedUnix.AsLocalTime(),
	})
}
