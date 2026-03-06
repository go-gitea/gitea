// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

type saveUploadChunkOptions struct {
	start    int64
	end      *int64
	checkMd5 bool
}

func makeTmpPathNameV3(runID int64) string {
	return fmt.Sprintf("tmp-upload/run-%d", runID)
}

func makeTmpPathNameV4(runID int64) string {
	return fmt.Sprintf("tmp-upload/run-%d-v4", runID)
}

func makeChunkFilenameV3(runID, artifactID, start int64, endPtr *int64) string {
	var end int64
	if endPtr != nil {
		end = *endPtr
	}
	return fmt.Sprintf("%d-%d-%d-%d.chunk", runID, artifactID, start, end)
}

func parseChunkFileItemV3(st storage.ObjectStorage, fpath string) (*chunkFileItem, error) {
	baseName := path.Base(fpath)
	if !strings.HasSuffix(baseName, ".chunk") {
		return nil, errSkipChunkFile
	}

	var item chunkFileItem
	var unusedRunID int64
	if _, err := fmt.Sscanf(baseName, "%d-%d-%d-%d.chunk", &unusedRunID, &item.ArtifactID, &item.Start, &item.End); err != nil {
		return nil, err
	}

	item.Path = fpath
	if item.End == 0 {
		fi, err := st.Stat(item.Path)
		if err != nil {
			return nil, err
		}
		item.Size = fi.Size()
		item.End = item.Start + item.Size - 1
	} else {
		item.Size = item.End - item.Start + 1
	}
	return &item, nil
}

func saveUploadChunkV3(st storage.ObjectStorage, ctx *ArtifactContext, artifact *actions.ActionArtifact,
	runID int64, opts saveUploadChunkOptions,
) (writtenSize int64, retErr error) {
	// build chunk store path
	storagePath := fmt.Sprintf("%s/%s", makeTmpPathNameV3(runID), makeChunkFilenameV3(runID, artifact.ID, opts.start, opts.end))

	// "end" is optional, so "contentSize=-1" means read until EOF
	contentSize := int64(-1)
	if opts.end != nil {
		contentSize = *opts.end - opts.start + 1
	}

	var r io.Reader = ctx.Req.Body
	var hasher hash.Hash
	if opts.checkMd5 {
		// use io.TeeReader to avoid reading all body to md5 sum.
		// it writes data to hasher after reading end
		// if hash is not matched, delete the read-end result
		hasher = md5.New()
		r = io.TeeReader(r, hasher)
	}
	// save chunk to storage
	writtenSize, err := st.Save(storagePath, r, contentSize)
	if err != nil {
		return 0, fmt.Errorf("save chunk to storage error: %v", err)
	}

	defer func() {
		if retErr != nil {
			if err := st.Delete(storagePath); err != nil {
				log.Error("Error deleting chunk: %s, %v", storagePath, err)
			}
		}
	}()

	if contentSize != -1 && writtenSize != contentSize {
		return writtenSize, fmt.Errorf("writtenSize %d does not match contentSize %d", writtenSize, contentSize)
	}
	if opts.checkMd5 {
		// check md5
		reqMd5String := ctx.Req.Header.Get(artifactXActionsResultsMD5Header)
		chunkMd5String := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
		log.Debug("[artifact] check chunk md5, sum: %s, header: %s", chunkMd5String, reqMd5String)
		// if md5 not match, delete the chunk
		if reqMd5String != chunkMd5String {
			return writtenSize, errors.New("md5 not match")
		}
	}
	log.Debug("[artifact] save chunk %s, size: %d, artifact id: %d, start: %d, size: %d", storagePath, writtenSize, artifact.ID, opts.start, contentSize)
	return writtenSize, nil
}

func saveUploadChunkV3GetTotalSize(st storage.ObjectStorage, ctx *ArtifactContext, artifact *actions.ActionArtifact, runID int64) (totalSize int64, _ error) {
	// parse content-range header, format: bytes 0-1023/146515
	contentRange := ctx.Req.Header.Get("Content-Range")
	var start, end int64
	if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &totalSize); err != nil {
		return 0, fmt.Errorf("parse content range error: %v", err)
	}
	_, err := saveUploadChunkV3(st, ctx, artifact, runID, saveUploadChunkOptions{start: start, end: &end, checkMd5: true})
	if err != nil {
		return 0, err
	}
	return totalSize, nil
}

// Returns uploaded length
func appendUploadChunkV3(st storage.ObjectStorage, ctx *ArtifactContext, artifact *actions.ActionArtifact, runID, start int64) (int64, error) {
	opts := saveUploadChunkOptions{start: start}
	if ctx.Req.ContentLength > 0 {
		end := start + ctx.Req.ContentLength - 1
		opts.end = &end
	}
	return saveUploadChunkV3(st, ctx, artifact, runID, opts)
}

type chunkFileItem struct {
	ArtifactID int64
	Path       string

	// these offset/size related fields might be missing when parsing, they will be filled in the listing functions
	Size  int64
	Start int64
	End   int64 // inclusive: Size=10, Start=0, End=9

	ChunkName string // v4 only
}

func listV3UnorderedChunksMapByRunID(st storage.ObjectStorage, runID int64) (map[int64][]*chunkFileItem, error) {
	storageDir := makeTmpPathNameV3(runID)
	var chunks []*chunkFileItem
	if err := st.IterateObjects(storageDir, func(fpath string, obj storage.Object) error {
		item, err := parseChunkFileItemV3(st, fpath)
		if errors.Is(err, errSkipChunkFile) {
			return nil
		} else if err != nil {
			return fmt.Errorf("unable to parse chunk name: %v", fpath)
		}
		chunks = append(chunks, item)
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

func listOrderedChunksForArtifact(st storage.ObjectStorage, runID, artifactID int64, blist *BlockList) ([]*chunkFileItem, error) {
	emptyListAsError := func(chunks []*chunkFileItem) ([]*chunkFileItem, error) {
		if len(chunks) == 0 {
			return nil, fmt.Errorf("no chunk found for artifact id: %d", artifactID)
		}
		return chunks, nil
	}

	storageDir := makeTmpPathNameV4(runID)
	var chunks []*chunkFileItem
	var chunkMapV4 map[string]*chunkFileItem

	if blist != nil {
		// make a dummy map for quick lookup of chunk names, the values are nil now and will be filled after iterating storage objects
		chunkMapV4 = map[string]*chunkFileItem{}
		for _, name := range blist.Latest {
			chunkMapV4[name] = nil
		}
	}

	if err := st.IterateObjects(storageDir, func(fpath string, obj storage.Object) error {
		item, err := parseChunkFileItemV4(st, artifactID, fpath)
		if errors.Is(err, errSkipChunkFile) {
			return nil
		} else if err != nil {
			return fmt.Errorf("unable to parse chunk name: %v", fpath)
		}

		// Single chunk upload with block id
		if _, ok := chunkMapV4[item.ChunkName]; ok {
			chunkMapV4[item.ChunkName] = item
		} else if chunkMapV4 == nil {
			if chunks != nil {
				return errors.New("blockmap is required for chunks > 1")
			}
			chunks = []*chunkFileItem{item}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if blist == nil && chunks == nil {
		chunkUnorderedItemsMapV3, err := listV3UnorderedChunksMapByRunID(st, runID)
		if err != nil {
			return nil, err
		}
		chunks = chunkUnorderedItemsMapV3[artifactID]
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].Start < chunks[j].Start
		})
		return emptyListAsError(chunks)
	}

	if len(chunks) == 0 && blist != nil {
		for i, name := range blist.Latest {
			chunk := chunkMapV4[name]
			if chunk == nil {
				return nil, fmt.Errorf("missing chunk (%d/%d): %s", i, len(blist.Latest), name)
			}
			chunks = append(chunks, chunk)
		}
	}
	for i, chunk := range chunks {
		if i == 0 {
			chunk.End += chunk.Size - 1
		} else {
			chunk.Start = chunkMapV4[blist.Latest[i-1]].End + 1
			chunk.End = chunk.Start + chunk.Size - 1
		}
	}
	return emptyListAsError(chunks)
}

func mergeChunksForRun(ctx *ArtifactContext, st storage.ObjectStorage, runID int64, artifactName string) error {
	// read all db artifacts by name
	artifacts, err := db.Find[actions.ActionArtifact](ctx, actions.FindArtifactsOptions{
		RunID:        runID,
		ArtifactName: artifactName,
	})
	if err != nil {
		return err
	}
	// read all uploading chunks from storage
	unorderedChunksMap, err := listV3UnorderedChunksMapByRunID(st, runID)
	if err != nil {
		return err
	}
	// range db artifacts to merge chunks
	for _, art := range artifacts {
		chunks, ok := unorderedChunksMap[art.ID]
		if !ok {
			log.Debug("artifact %d chunks not found", art.ID)
			continue
		}
		if err := mergeChunksForArtifact(ctx, chunks, st, art, ""); err != nil {
			return err
		}
	}
	return nil
}

func mergeChunksForArtifact(ctx *ArtifactContext, chunks []*chunkFileItem, st storage.ObjectStorage, artifact *actions.ActionArtifact, checksum string) error {
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
	shaPrefix := "sha256:"
	var hashSha256 hash.Hash
	if strings.HasPrefix(checksum, shaPrefix) {
		hashSha256 = sha256.New()
	} else if checksum != "" {
		setting.PanicInDevOrTesting("unsupported checksum format: %s, will skip the checksum verification", checksum)
	}
	if hashSha256 != nil {
		mergedReader = io.TeeReader(mergedReader, hashSha256)
	}

	// if chunk is gzip, use gz as extension
	// download-artifact action will use content-encoding header to decide if it should decompress the file
	extension := "chunk"
	if artifact.ContentEncoding == "gzip" {
		extension = "chunk.gz"
	}

	// save merged file
	storagePath := fmt.Sprintf("%d/%d/%d.%s", artifact.RunID%255, artifact.ID%255, time.Now().UnixNano(), extension)
	written, err := st.Save(storagePath, mergedReader, artifact.FileCompressedSize)
	if err != nil {
		return fmt.Errorf("save merged file error: %v", err)
	}
	if written != artifact.FileCompressedSize {
		return errors.New("merged file size is not equal to chunk length")
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

	if hashSha256 != nil {
		rawChecksum := hashSha256.Sum(nil)
		actualChecksum := hex.EncodeToString(rawChecksum)
		if !strings.HasSuffix(checksum, actualChecksum) {
			return fmt.Errorf("update artifact error checksum is invalid %v vs %v", checksum, actualChecksum)
		}
	}

	// save storage path to artifact
	log.Debug("[artifact] merge chunks to artifact: %d, %s, old:%s", artifact.ID, storagePath, artifact.StoragePath)
	// if artifact is already uploaded, delete the old file
	if artifact.StoragePath != "" {
		if err := st.Delete(artifact.StoragePath); err != nil {
			log.Warn("Error deleting old artifact: %s, %v", artifact.StoragePath, err)
		}
	}

	artifact.StoragePath = storagePath
	artifact.Status = actions.ArtifactStatusUploadConfirmed
	if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
		return fmt.Errorf("update artifact error: %v", err)
	}

	return nil
}
