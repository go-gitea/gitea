// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"gitea.dev/modules/setting"
)

func Init() error {
	if !setting.Audit.Enabled {
		return nil
	}

	return initAuditFile()
}
