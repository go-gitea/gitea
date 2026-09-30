// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddTimeIDCommentColumn(_ context.Context, x base.EngineMigration) error {
	type Comment struct {
		TimeID int64
	}

	if err := x.Sync(new(Comment)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
