// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

// AddReusableWorkflowFieldsToActionRunJob adds the ActionRunJob columns that describe the reusable workflow caller hierarchy,
// and the ActionRunAttemptJobIDIndex table backing run-wide AttemptJobID allocation.
func AddReusableWorkflowFieldsToActionRunJob(x *xorm.Engine) error {
	type ActionRunJob struct {
		IsReusableCaller        bool   `xorm:"index NOT NULL DEFAULT FALSE"`
		ParentCallerJobID       int64  `xorm:"index NOT NULL DEFAULT 0"`
		CallUses                string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
		CallSecrets             string `xorm:"LONGTEXT"`
		CallPayload             string `xorm:"LONGTEXT"`
		IsCallerExpanded        bool   `xorm:"NOT NULL DEFAULT FALSE"`
		ReusableWorkflowContent []byte `xorm:"LONGBLOB"`
	}

	type ActionRunAttemptJobIDIndex db.ResourceIndex

	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob)); err != nil {
		return err
	}
	return x.Sync(new(ActionRunAttemptJobIDIndex))
}
