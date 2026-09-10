// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddLFSMirrorColumns(_ context.Context, x base.EngineMigration) error {
	type Mirror struct {
		LFS         bool   `xorm:"lfs_enabled NOT NULL DEFAULT false"`
		LFSEndpoint string `xorm:"lfs_endpoint TEXT"`
	}

	return x.Sync(new(Mirror))
}
