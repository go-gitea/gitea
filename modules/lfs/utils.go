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

// ReadLFSPointerFile will return a partially filled LFSMetaObject if the provided reader is a pointer file
func ReadLFSPointerFile(reader io.Reader) (*models.LFSMetaObject, *[]byte) {
	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	buf = buf[:n]

	isTextFile := base.IsTextFile(buf)
	if isTextFile && setting.LFS.StartServer {
		return IsLFSPointerFile(&buf), &buf
	}

	return nil, &buf
}

// IsLFSPointerFile will return a partially filled LFSMetaObject if the provided byte slice is a pointer file
func IsLFSPointerFile(buf *[]byte) *models.LFSMetaObject {
	if setting.LFS.StartServer {
		headString := string(*buf)
		if strings.HasPrefix(headString, models.LFSMetaFileIdentifier) {
			splitLines := strings.Split(headString, "\n")
			if len(splitLines) >= 3 {
				oid := strings.TrimPrefix(splitLines[1], models.LFSMetaFileOidPrefix)
				size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
				if len(oid) == 64 && err == nil {
					contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
					meta := &models.LFSMetaObject{Oid: oid, Size: size}
					if contentStore.Exists(meta) {
						return meta
					}
				}
			}
		}
	}
	return nil
}
