// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"io"

	"code.gitea.io/gitea/modules/setting"
)

// ObjectStorage represents an object storage to handle a bucket and files
type ObjectStorage interface {
	Save(path string, r io.Reader) (int64, error)
	Open(path string) (io.ReadCloser, error)
	Delete(path string) error
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
)

// Init init the stoarge
func Init() error {
	var err error
	Attachments, err = NewLocalStorage(setting.AttachmentPath)
	if err != nil {
		return err
	}

	return nil
}
