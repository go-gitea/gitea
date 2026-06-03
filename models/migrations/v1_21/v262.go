// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import "gitea.dev/models/db"

func AddTriggerEventToActionRun(x db.EngineMigration) error {
	type ActionRun struct {
		TriggerEvent string
	}

	return x.Sync(new(ActionRun))
}
