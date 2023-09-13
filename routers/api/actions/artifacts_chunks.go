// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

func saveUploadChunk(st storage.ObjectStorage, ctx *ArtifactContext,
	artifact *actions.ActionArtifact,
	contentSize, runID int64,
) (int64, error) {
	// parse content-range header, format: bytes 0-1023/146515
	contentRange := ctx.Req.Header.Get("Content-Range")
	start, end, length := int64(0), int64(0), int64(0)
	if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &length); err != nil {
		return -1, fmt.Errorf("parse content range error: %v", err)
	}
	// build chunk store path
	storagePath := fmt.Sprintf("tmp%d/%d-%d-%d.chunk", runID, artifact.ID, start, end)
	// use io.TeeReader to avoid reading all body to md5 sum.
	// it writes data to hasher after reading end
	// if hash is not matched, delete the read-end result
	hasher := md5.New()
	r := io.TeeReader(ctx.Req.Body, hasher)
	// save chunk to storage
	writtenSize, err := st.Save(storagePath, r, -1)
	if err != nil {
		return -1, fmt.Errorf("save chunk to storage error: %v", err)
	}
	// check md5
	reqMd5String := ctx.Req.Header.Get(artifactXActionsResultsMD5Header)
	chunkMd5String := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	log.Info("[artifact] check chunk md5, sum: %s, header: %s", chunkMd5String, reqMd5String)
	// if md5 not match, delete the chunk
	if reqMd5String != chunkMd5String || writtenSize != contentSize {
		if err := st.Delete(storagePath); err != nil {
			log.Error("Error deleting chunk: %s, %v", storagePath, err)
		}
		return -1, fmt.Errorf("md5 not match")
	}
	log.Info("[artifact] save chunk %s, size: %d, artifact id: %d, start: %d, end: %d",
		storagePath, contentSize, artifact.ID, start, end)
	// return chunk total size
	return length, nil
}

type chunkFileItem struct {
	ArtifactID int64
	Start      int64
	End        int64
	Path       string
}

func listChunksByRunID(st storage.ObjectStorage, runID int64) (map[int64][]*chunkFileItem, error) {
	storageDir := fmt.Sprintf("tmp%d", runID)
	var chunks []*chunkFileItem
	if err := st.IterateObjects(storageDir, func(path string, obj storage.Object) error {
		item := chunkFileItem{Path: path}
		if _, err := fmt.Sscanf(path, filepath.Join(storageDir, "%d-%d-%d.chunk"), &item.ArtifactID, &item.Start, &item.End); err != nil {
			return fmt.Errorf("parse content range error: %v", err)
		}
		chunks = append(chunks, &item)
		return nil
	}); err != nil {
		return nil, err
	}
	// chunks group by artifact id
	chunksMap := make(map[int64][]*chunkFileItem)
	for _, c := range chunks {
		chunksMap[c.ArtifactID] = append(chunksMap[c.ArtifactID], c)
	}
	return chunksMap, nil
}

func mergeChunksForRun(ctx *ArtifactContext, st storage.ObjectStorage, runID int64, artifactName string) error {
	// read all db artifacts by name
	artifacts, err := actions.ListArtifactsByRunIDAndName(ctx, runID, artifactName)
	if err != nil {
		return err
	}
	// read all uploading chunks from storage
	chunksMap, err := listChunksByRunID(st, runID)
	if err != nil {
		return err
	}
	// range db artifacts to merge chunks
	for _, art := range artifacts {
		chunks, ok := chunksMap[art.ID]
		if !ok {
			log.Debug("artifact %d chunks not found", art.ID)
			continue
		}
		if err := mergeChunksForArtifact(ctx, chunks, st, art); err != nil {
			return err
		}
	}
	return nil
}

func mergeChunksForArtifact(ctx *ArtifactContext, chunks []*chunkFileItem, st storage.ObjectStorage, artifact *actions.ActionArtifact) error {
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Start < chunks[j].Start
	})
	allChunks := make([]*chunkFileItem, 0)
	startAt := int64(-1)
	// check if all chunks are uploaded and in order and clean repeated chunks
	for _, c := range chunks {
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
		log.Debug("[artifact] chunks are not uploaded completely, artifact_id: %d", artifact.ID)
		return nil
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
		var err error
		if readCloser, err = st.Open(c.Path); err != nil {
			return fmt.Errorf("open chunk error: %v, %s", err, c.Path)
		}
		readers = append(readers, readCloser)
	}
	mergedReader := io.MultiReader(readers...)

	// if chunk is gzip, use gz as extension
	// download-artifact action will use content-encoding header to decide if it should decompress the file
	extension := "chunk"
	if artifact.ContentEncoding == "gzip" {
		extension = "chunk.gz"
	}

	// save merged file
	storagePath := fmt.Sprintf("%d/%d/%d.%s", artifact.RunID%255, artifact.ID%255, time.Now().UnixNano(), extension)
	written, err := st.Save(storagePath, mergedReader, -1)
	if err != nil {
		return fmt.Errorf("save merged file error: %v", err)
	}
	if written != artifact.FileCompressedSize {
		return fmt.Errorf("merged file size is not equal to chunk length")
	}

	defer func() {
		closeReaders() // close before delete
		// drop chunks
		for _, c := range chunks {
			if err := st.Delete(c.Path); err != nil {
				log.Warn("Error deleting chunk: %s, %v", c.Path, err)
			}
		}
	}()

	// save storage path to artifact
	log.Debug("[artifact] merge chunks to artifact: %d, %s", artifact.ID, storagePath)
	artifact.StoragePath = storagePath
	artifact.Status = int64(actions.ArtifactStatusUploadConfirmed)
	if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
		return fmt.Errorf("update artifact error: %v", err)
	}

	return nil
}
