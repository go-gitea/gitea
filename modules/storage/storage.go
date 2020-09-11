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

func initStorage(storageCfg setting.Storage) (ObjectStorage, error) {
	var err error
	var s ObjectStorage
	switch storageCfg.StoreType {
	case "local":
		s, err = NewLocalStorage(storageCfg.Path)
	case "minio":
		minio := storageCfg.Minio
		s, err = NewMinioStorage(
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
		return nil, fmt.Errorf("Unsupported attachment store type: %s", storageCfg.StoreType)
	}

	if err != nil {
		return nil, err
	}

	return s, nil
}

func initAttachments() (err error) {
	Attachments, err = initStorage(setting.Attachment.Storage)
	return
}

func initLFS() (err error) {
	LFS, err = initStorage(setting.LFS.Storage)
	return
}
