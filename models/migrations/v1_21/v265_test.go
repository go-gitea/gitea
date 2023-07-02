// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_BranchColumnNameCollation(t *testing.T) {
	type Branch struct {
		ID     int64
		RepoID int64  `xorm:"Unique(s)"`
		Name   string `xorm:"Unique(s) NOT NULL"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Branch))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, BranchColumnNameCollation(x))
}
