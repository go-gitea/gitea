// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"bufio"
	"context"
)

// batchCheck represents an active `git cat-file --batch-check` invocation
// paired with the pipes that feed/read from it. Call Close to release resources.
type batchCheck struct {
	cancel context.CancelFunc
	reader *bufio.Reader
	writer WriteCloserError
}

// NewBatchCheck creates a cat-file --batch-check process for the provided repository path.
// The returned Batch must be closed once the caller has finished with it.
func NewObjectInfoPool(ctx context.Context, repoPath string) (ObjectInfoPool, error) {
	if err := EnsureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var check batchCheck
	check.writer, check.reader, check.cancel = catFileBatchCheck(ctx, repoPath)
	return &check, nil
}

func (b *batchCheck) ObjectInfo(refName string) (*ObjectInfo, error) {
	_, err := b.writer.Write([]byte(refName + "\n"))
	if err != nil {
		return nil, err
	}

	var objInfo ObjectInfo
	var oid []byte
	oid, objInfo.Type, objInfo.Size, err = ReadBatchLine(b.reader)
	if err != nil {
		return nil, err
	}
	objInfo.ID = string(oid)
	return &objInfo, nil
}

// Close stops the underlying git cat-file process and releases held resources.
func (b *batchCheck) Close() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.writer != nil {
		_ = b.writer.Close()
	}
}

// batch represents an active `git cat-file --batch` invocation
// paired with the pipes that feed/read from it. Call Close to release resources.
type batch struct {
	cancel context.CancelFunc
	reader *bufio.Reader
	writer WriteCloserError
}

// NewBatch creates a new cat-file --batch process for the provided repository path.
// The returned Batch must be closed once the caller has finished with it.
func NewObjectPool(ctx context.Context, repoPath string) (ObjectPool, error) {
	if err := EnsureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var batch batch
	batch.writer, batch.reader, batch.cancel = catFileBatch(ctx, repoPath)
	return &batch, nil
}

func (b *batch) Object(refName string) (*ObjectInfo, *bufio.Reader, error) {
	_, err := b.writer.Write([]byte(refName + "\n"))
	if err != nil {
		return nil, nil, err
	}

	var obj ObjectInfo
	var oid []byte
	oid, obj.Type, obj.Size, err = ReadBatchLine(b.reader)
	if err != nil {
		return nil, nil, err
	}
	obj.ID = string(oid)

	return &obj, b.reader, nil
}

func (b *batch) Close() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.writer != nil {
		_ = b.writer.Close()
	}
}
