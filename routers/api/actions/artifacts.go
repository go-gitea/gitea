// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"compress/gzip"
	gocontext "context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
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

// const artifactXActionsResultsCRC64Header = "x-actions-results-crc64"

const artifactRouteBase = "/_apis/pipelines/workflows/{taskID}/artifacts"

func ArtifactsRoutes(goctx gocontext.Context, prefix string) *web.Route {
	m := web.NewRoute()
	m.Use(withContexter(goctx))

	r := artifactRoutes{
		prefix: prefix,
		fs:     storage.ActionsArtifacts,
	}

	// retrieve, list and confirm artifacts
	m.Post(artifactRouteBase, r.getUploadArtifactURL)
	m.Get(artifactRouteBase, r.listArtifacts)
	m.Patch(artifactRouteBase, r.comfirmUploadArtifact)

	// handle container artifacts
	m.Put(artifactRouteBase+"/{artifactID}/upload", r.uploadArtifact)
	m.Get(artifactRouteBase+"/{artifactID}/path", r.getDownloadArtifactURL)
	m.Get(artifactRouteBase+"/{artifactID}/download", r.downloadArtifact)

	return m
}

// withContexter initializes a package context for a request.
func withContexter(ctx gocontext.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.Context{
				Resp: context.NewResponse(resp),
				Data: map[string]interface{}{},
			}
			defer ctx.Close()

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
			ctx.Data["task"] = task

			ctx.Req = context.WithContext(req, &ctx)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

type artifactRoutes struct {
	prefix string
	fs     storage.ObjectStorage
}

func (ar artifactRoutes) buildArtifactURL(taskID, artifactID int64, suffix string) (string, error) {
	uploadURL := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ar.prefix, "/") +
		strings.ReplaceAll(artifactRouteBase, "{taskID}", strconv.FormatInt(taskID, 10)) +
		"/" + strconv.FormatInt(artifactID, 10) + "/" + suffix
	u, err := url.Parse(uploadURL)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// getUploadArtifactURL generates a URL for uploading an artifact
func (ar artifactRoutes) getUploadArtifactURL(ctx *context.Context) {
	task, ok := ctx.Data["task"].(*actions.ActionTask)
	if !ok {
		log.Error("Error getting task in context")
		ctx.Error(http.StatusInternalServerError, "Error getting task in context")
		return
	}
	taskID := ctx.ParamsInt64("taskID")
	if task.ID != taskID {
		log.Error("Error task id not match")
		ctx.Error(http.StatusInternalServerError, "Error task id not match")
		return
	}

	artifact, err := actions.CreateArtifact(ctx, task)
	if err != nil {
		log.Error("Error creating artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	url, err := ar.buildArtifactURL(taskID, artifact.ID, "upload")
	if err != nil {
		log.Error("Error parsing upload URL: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	log.Debug("[artifact] get upload url: %s, artifact id: %d", url, artifact.ID)

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"fileContainerResourceUrl": url,
	})
}

// getUploadFileSize returns the size of the file to be uploaded.
// The raw size is the size of the file as reported by the header X-TFS-FileLength.
func (ar artifactRoutes) getUploadFileSize(ctx *context.Context) (int64, int64, error) {
	contentLength := ctx.Req.ContentLength
	xTfsLength, _ := strconv.ParseInt(ctx.Req.Header.Get(artifactXTfsFileLengthHeader), 10, 64)
	if xTfsLength > 0 {
		return xTfsLength, contentLength, nil
	}
	return contentLength, contentLength, nil
}

type hashReader struct {
	Reader io.Reader
	Hasher hash.Hash
}

func (hr *hashReader) Read(p []byte) (n int, err error) {
	n, err = hr.Reader.Read(p)
	if err == nil {
		hr.Hasher.Write(p[:n])
	}
	return n, err
}

func (hr *hashReader) Match(md5Str string) bool {
	md5Hash := hr.Hasher.Sum(nil)
	md5String := base64.StdEncoding.EncodeToString(md5Hash)
	return md5Str == md5String
}

func (ar artifactRoutes) saveUploadChunk(ctx *context.Context,
	artifact *actions.ActionArtifact,
	contentSize, taskID int64,
) (int64, error) {
	contentRange := ctx.Req.Header.Get("Content-Range")
	start, end, length := int64(0), int64(0), int64(0)
	if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &length); err != nil {
		return -1, fmt.Errorf("parse content range error: %v", err)
	}

	storagePath := fmt.Sprintf("tmp%d/%d-%d-%d.chunk", taskID, artifact.ID, start, end)

	// use hashReader to avoid reading all body to md5 sum.
	// it writes data to hasher after reading end
	// if hash is not matched, delete the read-end result
	r := &hashReader{
		Reader: ctx.Req.Body,
		Hasher: md5.New(),
	}

	// save chunk to storage
	if _, err := ar.fs.Save(storagePath, r, -1); err != nil {
		return -1, fmt.Errorf("save chunk to storage error: %v", err)
	}

	// check md5
	if !r.Match(ctx.Req.Header.Get(artifactXActionsResultsMD5Header)) {
		if err := ar.fs.Delete(storagePath); err != nil {
			log.Error("Error deleting chunk: %s, %v", storagePath, err)
		}
		return -1, fmt.Errorf("md5 not match")
	}

	log.Debug("[artifact] save chunk %s, size: %d, artifact id: %d, start: %d, end: %d",
		storagePath, contentSize, artifact.ID, start, end)

	return length, nil
}

var invalidArtifactNameChars = []string{"\\", "/", "\"", ":", "<", ">", "|", "*", "?", "\r", "\n"}

// checkArtifactName checks if the artifact name contains invalid characters.
// If the name contains invalid characters, an error is returned.
// The rules are from https://github.com/actions/toolkit/blob/main/packages/artifact/src/internal/path-and-artifact-name-validation.ts#L32
func checkArtifactName(name string) error {
	for _, c := range invalidArtifactNameChars {
		if strings.Contains(name, c) {
			return fmt.Errorf("artifact name contains invalid character %s", c)
		}
	}
	return nil
}

func (ar artifactRoutes) uploadArtifact(ctx *context.Context) {
	artifactID := ctx.ParamsInt64("artifactID")

	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	// itemPath is generated from upload-artifact action
	// it's formatted as {artifact_name}/{artfict_path_in_runner}
	itemPath := util.PathJoinRel(ctx.Req.URL.Query().Get("itemPath"))
	taskID := ctx.ParamsInt64("taskID")
	artifactName := strings.Split(itemPath, "/")[0]
	if err = checkArtifactName(artifactName); err != nil {
		log.Error("Error checking artifact name: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
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
	chunkAllLength, err := ar.saveUploadChunk(ctx, artifact, contentLength, taskID)
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
		artifact.FileGzipSize = chunkAllLength
		artifact.ContentEncnoding = ctx.Req.Header.Get("Content-Encoding")
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
func (ar artifactRoutes) comfirmUploadArtifact(ctx *context.Context) {
	taskID := ctx.ParamsInt64("taskID")
	if err := ar.mergeArtifactChunks(ctx, taskID); err != nil {
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

func (ar artifactRoutes) mergeArtifactChunks(ctx *context.Context, taskID int64) error {
	storageDir := fmt.Sprintf("tmp%d", taskID)
	var chunks []*chunkItem
	if err := ar.fs.IterateObjects(storageDir, func(path string, obj storage.Object) error {
		defer obj.Close()
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
		if startAt+1 != artifact.FileGzipSize {
			log.Debug("[artifact] chunks are not uploaded completely, artifact_id: %d", artifactID)
			break
		}

		// use multiReader
		readers := make([]io.Reader, 0, len(allChunks))
		for _, c := range allChunks {
			reader, err := ar.fs.Open(c.Path)
			if err != nil {
				return fmt.Errorf("open chunk error: %v, %s", err, c.Path)
			}
			readers = append(readers, reader)
		}
		mergedReader := io.MultiReader(readers...)

		// if chunk is gzip, decompress it
		if artifact.ContentEncnoding == "gzip" {
			var err error
			mergedReader, err = gzip.NewReader(mergedReader)
			if err != nil {
				return fmt.Errorf("gzip reader error: %v", err)
			}
		}

		// save merged file
		storagePath := fmt.Sprintf("%d/%d/%d.chunk", (taskID+artifactID)%255, artifact.FileSize%255, time.Now().UnixNano())
		written, err := ar.fs.Save(storagePath, mergedReader, -1)
		if err != nil {
			return fmt.Errorf("save merged file error: %v", err)
		}
		if written != artifact.FileSize {
			return fmt.Errorf("merged file size is not equal to chunk length")
		}

		// close readers
		for _, r := range readers {
			r.(io.Closer).Close()
		}

		// save storage path to artifact
		log.Debug("[artifact] merge chunks to artifact: %d, %s", artifact.ID, storagePath)
		artifact.StoragePath = storagePath
		artifact.UploadStatus = actions.ArtifactUploadStatusConfirmed
		if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
			return fmt.Errorf("update artifact error: %v", err)
		}

		// drop chunks
		for _, c := range cs {
			if err := ar.fs.Delete(c.Path); err != nil {
				return fmt.Errorf("delete chunk file error: %v", err)
			}
		}
	}
	return nil
}

func (ar artifactRoutes) listArtifacts(ctx *context.Context) {
	task, ok := ctx.Data["task"].(*actions.ActionTask)
	if !ok {
		log.Error("Error getting task in context")
		ctx.Error(http.StatusInternalServerError, "Error getting task in context")
		return
	}
	taskID := ctx.ParamsInt64("taskID")
	if task.ID != taskID {
		log.Error("Error task id not match")
		ctx.Error(http.StatusInternalServerError, "Error task id not match")
		return
	}

	artficats, err := actions.ListArtifactByJobID(ctx, task.JobID)
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	artficatsData := make([]map[string]interface{}, 0, len(artficats))
	for _, a := range artficats {
		url, err := ar.buildArtifactURL(taskID, a.ID, "path")
		if err != nil {
			log.Error("Error parsing artifact URL: %v", err)
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		artficatsData = append(artficatsData, map[string]interface{}{
			"name":                     a.ArtifactName,
			"fileContainerResourceUrl": url,
		})
	}
	respData := map[string]interface{}{
		"count": len(artficatsData),
		"value": artficatsData,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (ar artifactRoutes) getDownloadArtifactURL(ctx *context.Context) {
	artifactID := ctx.ParamsInt64("artifactID")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	taskID := ctx.ParamsInt64("taskID")
	url, err := ar.buildArtifactURL(taskID, artifact.ID, "download")
	if err != nil {
		log.Error("Error parsing download URL: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	itemPath := util.PathJoinRel(ctx.Req.URL.Query().Get("itemPath"))
	artifactData := map[string]string{
		"path":            util.PathJoinRel(itemPath, artifact.ArtifactPath),
		"itemType":        "file",
		"contentLocation": url,
	}
	respData := map[string]interface{}{
		"value": []interface{}{artifactData},
	}
	ctx.JSON(http.StatusOK, respData)
}

func (ar artifactRoutes) downloadArtifact(ctx *context.Context) {
	artifactID := ctx.ParamsInt64("artifactID")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	taskID := ctx.ParamsInt64("taskID")
	if artifact.JobID != taskID {
		log.Error("Error dismatch taskID and artifactID, task: %v, artifact: %v", taskID, artifactID)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	fd, err := ar.fs.Open(artifact.StoragePath)
	if err != nil {
		log.Error("Error opening file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if strings.HasSuffix(artifact.ArtifactPath, ".gz") {
		ctx.Resp.Header().Set("Content-Encoding", "gzip")
	}
	_, err = io.Copy(ctx.Resp, fd)
	if err != nil {
		log.Error("Error copying file to response: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
}
