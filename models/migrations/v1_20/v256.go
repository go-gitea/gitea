// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateActionArtifactTable(x *xorm.Engine) error {
	type ActionArtifact struct {
		ID               int64 `xorm:"pk autoincr"`
		JobID            int64 `xorm:"index"`
		RunnerID         int64
		RepoID           int64 `xorm:"index"`
		OwnerID          int64
		CommitSHA        string
		StoragePath      string // The path to the artifact in the storage
		FileSize         int64
		FileGzipSize     int64
		ContentEncnoding string             // The content encoding of the artifact, such as gzip
		ArtifactPath     string             // The path to the artifact when runner uploads it
		ArtifactName     string             // The name of the artifact when runner uploads it
		UploadStatus     int64              `xorm:"index"`
		Created          timeutil.TimeStamp `xorm:"created"`
		Updated          timeutil.TimeStamp `xorm:"updated index"`
	}

	return x.Sync(new(ActionArtifact))
}
