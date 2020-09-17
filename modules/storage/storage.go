// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

var (
	// ErrURLNotSupported represents url is not supported
	ErrURLNotSupported = errors.New("url method not supported")
)

// Object represents the object on the storage
type Object interface {
	io.ReadCloser
	io.Seeker
}

// ObjectInfo represents the object info on the storage
type ObjectInfo interface {
	Name() string
	Size() int64
	ModTime() time.Time
}

// ObjectStorage represents an object storage to handle a bucket and files
type ObjectStorage interface {
	Open(path string) (Object, error)
	Save(path string, r io.Reader) (int64, error)
	Stat(path string) (ObjectInfo, error)
	Delete(path string) error
	URL(path, name string) (*url.URL, error)
}

// Copy copys a file from source ObjectStorage to dest ObjectStorage
func Copy(dstStorage ObjectStorage, dstPath string, srcStorage ObjectStorage, srcPath string) (int64, error) {
	f, err := srcStorage.Open(srcPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return dstStorage.Save(dstPath, f)
}

var (
	// Attachments represents attachments storage
	Attachments ObjectStorage

	// LFS represents lfs storage
	LFS ObjectStorage
)

// Init init the stoarge
func Init() error {
	if err := initAttachments(); err != nil {
		return err
	}

	return initLFS()
}

func initAttachments() error {
	var err error
	switch setting.Attachment.StoreType {
	case "local":
		Attachments, err = NewLocalStorage(setting.Attachment.Path)
	case "minio":
		minio := setting.Attachment.Minio
		Attachments, err = NewMinioStorage(
			context.Background(),
			minio.Endpoint,
			minio.AccessKeyID,
			minio.SecretAccessKey,
			minio.Bucket,
			minio.Location,
			minio.BasePath,
			minio.UseSSL,
		)
	default:
		return fmt.Errorf("Unsupported attachment store type: %s", setting.Attachment.StoreType)
	}

	if err != nil {
		return err
	}

	return nil
}

func initLFS() error {
	var err error
	switch setting.LFS.StoreType {
	case "local":
		LFS, err = NewLocalStorage(setting.LFS.ContentPath)
	case "minio":
		minio := setting.LFS.Minio
		LFS, err = NewMinioStorage(
			context.Background(),
			minio.Endpoint,
			minio.AccessKeyID,
			minio.SecretAccessKey,
			minio.Bucket,
			minio.Location,
			minio.BasePath,
			minio.UseSSL,
		)
	default:
		return fmt.Errorf("Unsupported LFS store type: %s", setting.LFS.StoreType)
	}

	if err != nil {
		return err
	}

	return nil
}
