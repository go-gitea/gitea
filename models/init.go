// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"

	"gitea.dev/models/unit"
)

// Init initialize model
func Init(ctx context.Context) error {
	return unit.LoadUnitConfig()
}
