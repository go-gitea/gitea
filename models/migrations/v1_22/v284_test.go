// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func TestAlterBranchNameCollation(t *testing.T) {
	type Branch struct {
		ID     int64
		RepoID int64  `xorm:"UNIQUE(s)"`
		Name   string `xorm:"UNIQUE(s) NOT NULL"`
	}

	x, deferable := base.PrepareTestEnv(t, 0, new(Branch))
	defer deferable()

	assert.NoError(t, AlterBranchNameCollation(x))
}
