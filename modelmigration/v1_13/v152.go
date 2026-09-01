// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddTrustModelToRepository(_ context.Context, x base.EngineMigration) error {
	type Repository struct {
		TrustModel int
	}
	return x.Sync(new(Repository))
}
