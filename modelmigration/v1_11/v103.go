// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddWhitelistDeployKeysToBranches(_ context.Context, x base.EngineMigration) error {
	type ProtectedBranch struct {
		ID                  int64
		WhitelistDeployKeys bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
