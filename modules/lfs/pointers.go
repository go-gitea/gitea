// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"io"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// ReadPointerFile will return a partially filled LFSMetaObject if the provided reader is a pointer file
func ReadPointerFile(reader io.Reader) (*models.LFSMetaObject, *[]byte) {
	if !setting.LFS.StartServer {
		return nil, nil
	}

	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	buf = buf[:n]

	if isTextFile := base.IsTextFile(buf); !isTextFile {
		return nil, nil
	}

	return models.IsPointerFileAndStored(&buf), &buf
}

// ReadMetaObject will read a models.LFSMetaObject and return a reader
func ReadMetaObject(meta *models.LFSMetaObject) (io.ReadCloser, error) {
	contentStore := &models.ContentStore{ObjectStorage: storage.LFS}
	return contentStore.Get(meta, 0)
}
