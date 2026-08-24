// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddWorkflowCallOriginalEventSupportToActionRunner(_ context.Context, x base.EngineMigration) error {
	type ActionRunner struct {
		HasWorkflowCallOriginalEventSupport bool `xorm:"has_workflow_call_original_event_support NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunner))
	return err
}
