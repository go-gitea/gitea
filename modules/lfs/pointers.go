// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
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

	return IsPointerFile(&buf), &buf
}

// IsPointerFile will return a partially filled LFSMetaObject if the provided byte slice is a pointer file
func IsPointerFile(buf *[]byte) *models.LFSMetaObject {
	if !setting.LFS.StartServer {
		return nil
	}

	headString := string(*buf)
	if !strings.HasPrefix(headString, models.LFSMetaFileIdentifier) {
		return nil
	}

	splitLines := strings.Split(headString, "\n")
	if len(splitLines) < 3 {
		return nil
	}

	oid := strings.TrimPrefix(splitLines[1], models.LFSMetaFileOidPrefix)
	size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
	if len(oid) != 64 || err != nil {
		return nil
	}

	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	meta := &models.LFSMetaObject{Oid: oid, Size: size}
	if !contentStore.Exists(meta) {
		return nil
	}

	return meta
}

// ReadMetaObject will read a models.LFSMetaObject and return a reader
func ReadMetaObject(meta *models.LFSMetaObject) (io.ReadCloser, error) {
	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	return contentStore.Get(meta, 0)
}
