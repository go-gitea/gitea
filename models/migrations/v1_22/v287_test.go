// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_UpdateBadgeColName(t *testing.T) {
	type Badge struct {
		ID          int64  `xorm:"pk autoincr"`
		Slug        string `xorm:"UNIQUE"`
		Description string
		ImageURL    string
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(BadgeUnique), new(Badge))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// TODO: insert example badges

	if err := UseSlugInsteadOfIDForBadges(x); err != nil {
		assert.NoError(t, err)
		return
	}

	// TODO: check if badges have been updated
}
