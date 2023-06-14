// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"errors"
	"io"
	"os"

	packages_model "code.gitea.io/gitea/models/packages"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var (
	// errWriteAfterRead occurs if Write is called after a read operation
	errWriteAfterRead = errors.New("write is unsupported after a read operation")
	// errOffsetMissmatch occurs if the file offset is different than the model
	errOffsetMissmatch = errors.New("offset mismatch between file and model")
)

// BlobUploader handles chunked blob uploads
type BlobUploader struct {
	*packages_model.PackageBlobUpload
	*packages_module.MultiHasher
	file    *os.File
	reading bool
}

func buildFilePath(id string) string {
	return util.FilePathJoinAbs(setting.Packages.ChunkedUploadPath, id)
}

// NewBlobUploader creates a new blob uploader for the given id
func NewBlobUploader(ctx context.Context, id string) (*BlobUploader, error) {
	model, err := packages_model.GetBlobUploadByID(ctx, id)
	if err != nil {
		return nil, err
	}

	hash := packages_module.NewMultiHasher()
	if len(model.HashStateBytes) != 0 {
		if err := hash.UnmarshalBinary(model.HashStateBytes); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(buildFilePath(model.ID), os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, err
	}

	return &BlobUploader{
		model,
		hash,
		f,
		false,
	}, nil
}

// Close implements io.Closer
func (u *BlobUploader) Close() error {
	return u.file.Close()
}

// Append appends a chunk of data and updates the model
func (u *BlobUploader) Append(ctx context.Context, r io.Reader) error {
	if u.reading {
		return errWriteAfterRead
	}

	offset, err := u.file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if offset != u.BytesReceived {
		return errOffsetMissmatch
	}

	n, err := io.Copy(io.MultiWriter(u.file, u.MultiHasher), r)
	if err != nil {
		return err
	}

	// fast path if nothing was written
	if n == 0 {
		return nil
	}

	u.BytesReceived += n

	u.HashStateBytes, err = u.MultiHasher.MarshalBinary()
	if err != nil {
		return err
	}

	return packages_model.UpdateBlobUpload(ctx, u.PackageBlobUpload)
}

func (u *BlobUploader) Size() int64 {
	return u.BytesReceived
}

// Read implements io.Reader
func (u *BlobUploader) Read(p []byte) (int, error) {
	if !u.reading {
		_, err := u.file.Seek(0, io.SeekStart)
		if err != nil {
			return 0, err
		}

		u.reading = true
	}

	return u.file.Read(p)
}

// Remove deletes the data and the model of a blob upload
func RemoveBlobUploadByID(ctx context.Context, id string) error {
	if err := packages_model.DeleteBlobUploadByID(ctx, id); err != nil {
		return err
	}

	err := os.Remove(buildFilePath(id))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
