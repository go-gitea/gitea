// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_CreateTableIssueDevLink(t *testing.T) {
	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0)
	defer deferable()

	assert.NoError(t, CreateTableIssueDevLink(x))
}
