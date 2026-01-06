// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"bufio"
	"context"

	"code.gitea.io/gitea/modules/log"
)

// batch represents an active `git cat-file --batch` invocation
// paired with the pipes that feed/read from it. Call Close to release resources.
type batch struct {
	cancel context.CancelFunc
	reader *bufio.Reader
	writer WriteCloserError
	inUse  bool
}

func (b *batch) Close() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	if b.writer != nil {
		_ = b.writer.Close()
		b.writer = nil
	}
}

type batchObjectPool struct {
	ctx         context.Context
	repoPath    string
	batches     []*batch
	batchChecks []*batch
}

// NewBatchObjectPool creates a new ObjectPool that uses git cat-file --batch and --batch-check
// to read objects from the repository at repoPath.
func NewBatchObjectPool(ctx context.Context, repoPath string) (ObjectPool, error) {
	if err := EnsureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	return &batchObjectPool{
		ctx:      ctx,
		repoPath: repoPath,
	}, nil
}

func (b *batchObjectPool) getBatch() *batch {
	for _, batch := range b.batches {
		if !batch.inUse {
			batch.inUse = true
			return batch
		}
	}
	if len(b.batches) >= 1 {
		log.Warn("Opening temporary cat file batch for: %s", b.repoPath)
	}
	newBatch := b.newBatch()
	newBatch.inUse = true
	b.batches = append(b.batches, newBatch)
	return newBatch
}

// newBatch creates a new cat-file --batch process for the provided repository path.
// The returned Batch must be closed when objectPool closed.
func (b *batchObjectPool) newBatch() *batch {
	var batch batch
	batch.writer, batch.reader, batch.cancel = catFileBatch(b.ctx, b.repoPath)
	return &batch
}

func (b *batchObjectPool) getBatchCheck() *batch {
	for _, batch := range b.batchChecks {
		if !batch.inUse {
			batch.inUse = true
			return batch
		}
	}
	if len(b.batchChecks) >= 1 {
		log.Warn("Opening temporary cat file batch-check for: %s", b.repoPath)
	}
	newBatch := b.newBatchCheck()
	newBatch.inUse = true
	b.batchChecks = append(b.batchChecks, newBatch)
	return newBatch
}

// newBatchCheck creates a new cat-file --batch-check process for the provided repository path.
// The returned Batch must be closed when objectPool closed.
func (b *batchObjectPool) newBatchCheck() *batch {
	var check batch
	check.writer, check.reader, check.cancel = catFileBatchCheck(b.ctx, b.repoPath)
	return &check
}

func releaseBatchCheck(batch *batch) {
	if batch != nil {
		batch.inUse = false
	}
}

func (b *batchObjectPool) ObjectInfo(refName string) (*ObjectInfo, error) {
	batch := b.getBatchCheck()
	defer releaseBatchCheck(batch)

	_, err := batch.writer.Write([]byte(refName + "\n"))
	if err != nil {
		return nil, err
	}

	var objInfo ObjectInfo
	var oid []byte
	oid, objInfo.Type, objInfo.Size, err = ReadBatchLine(batch.reader)
	if err != nil {
		return nil, err
	}
	objInfo.ID = string(oid)
	return &objInfo, nil
}

type readCloser struct {
	*bufio.Reader
	batch *batch
}

func (rc *readCloser) Close() error {
	rc.batch.inUse = false
	return nil
}

func (b *batchObjectPool) Object(refName string) (*ObjectInfo, ReadCloseDiscarder, error) {
	batch := b.getBatch()

	_, err := batch.writer.Write([]byte(refName + "\n"))
	if err != nil {
		return nil, nil, err
	}

	var obj ObjectInfo
	var oid []byte
	oid, obj.Type, obj.Size, err = ReadBatchLine(batch.reader)
	if err != nil {
		return nil, nil, err
	}
	obj.ID = string(oid)

	return &obj, &readCloser{Reader: batch.reader, batch: batch}, nil
}

func (b *batchObjectPool) Close() {
	for _, batch := range b.batches {
		batch.Close()
	}
	b.batches = nil
	for _, batchCheck := range b.batchChecks {
		batchCheck.Close()
	}
	b.batchChecks = nil
}
