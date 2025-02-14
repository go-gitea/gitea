// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

func AlterActionArtifactTable(x *xorm.Engine) error {
	// ActionArtifact is a file that is stored in the artifact storage.
	type ActionArtifact struct {
		RunID        int64  `xorm:"index unique(runid_name_path)"` // The run id of the artifact
		ArtifactPath string `xorm:"index unique(runid_name_path)"` // The path to the artifact when runner uploads it
		ArtifactName string `xorm:"index unique(runid_name_path)"` // The name of the artifact when
	}

	return x.Sync(new(ActionArtifact))
}
