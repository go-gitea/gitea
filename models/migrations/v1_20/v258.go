// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateActionArtifactTable(x *xorm.Engine) error {
	// ActionArtifact is a file that is stored in the artifact storage.
	type ActionArtifact struct {
		ID                 int64 `xorm:"pk autoincr"`
		RunID              int64 `xorm:"index UNIQUE(runid_name)"` // The run id of the artifact
		RunnerID           int64
		RepoID             int64 `xorm:"index"`
		OwnerID            int64
		CommitSHA          string
		StoragePath        string             // The path to the artifact in the storage
		FileSize           int64              // The size of the artifact in bytes
		FileCompressedSize int64              // The size of the artifact in bytes after gzip compression
		ContentEncoding    string             // The content encoding of the artifact
		ArtifactPath       string             // The path to the artifact when runner uploads it
		ArtifactName       string             `xorm:"UNIQUE(runid_name)"` // The name of the artifact when runner uploads it
		Status             int64              `xorm:"index"`              // The status of the artifact
		CreatedUnix        timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix        timeutil.TimeStamp `xorm:"updated index"`
	}

	return x.Sync(new(ActionArtifact))
}
