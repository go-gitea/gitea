// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gitea.dev/modules/util"
)

// WithContext shares a configured backend while binding its I/O to this request.
// Never change the global backend's context: other uploads use it concurrently.
func WithContext(ctx context.Context, backend ObjectStorage) (ObjectStorage, error) {
	switch backend := backend.(type) {
	case *MinioStorage:
		scoped := *backend
		scoped.ctx = ctx
		return &scoped, nil
	case *AzureBlobStorage:
		scoped := *backend
		scoped.ctx = ctx
		return &scoped, nil
	case *LocalStorage:
		return &contextLocalStorage{LocalStorage: backend, ctx: ctx}, nil
	case discardStorage:
		return backend, nil
	default:
		return nil, fmt.Errorf("storage backend %T does not support request cancellation", backend)
	}
}

type contextLocalStorage struct {
	*LocalStorage
	ctx context.Context
}

func (s *contextLocalStorage) Stat(path string) (os.FileInfo, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	return s.LocalStorage.Stat(path)
}

// Use a private temporary file so cancellation never removes a shared object.
// Local filesystem syscalls themselves cannot be interrupted by a Go context.
func (s *contextLocalStorage) Save(path string, r io.Reader, _ int64) (int64, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	p := s.buildLocalPath(path)
	if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(s.tmpdir, os.ModePerm); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(s.tmpdir, "upload-*")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tmp.Close()
		_ = util.Remove(tmp.Name())
	}()
	n, err := io.Copy(tmp, &storageContextReader{ctx: s.ctx, reader: r})
	if err != nil {
		return n, err
	}
	if err := tmp.Close(); err != nil {
		return n, err
	}
	if err := s.ctx.Err(); err != nil {
		return n, err
	}
	if err := util.ApplyUmask(tmp.Name(), os.ModePerm&0o666); err != nil {
		return n, err
	}
	return n, util.Rename(tmp.Name(), p)
}

type storageContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *storageContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
