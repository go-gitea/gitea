// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/migrationtest"

	"github.com/stretchr/testify/assert"
)

func Test_AddHTTPSDeployKeyTable(t *testing.T) {
	// Start from an empty DB — nothing depends on prior migration state.
	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := AddHTTPSDeployKeyTable(x); err != nil {
		assert.NoError(t, err)
		return
	}

	type HTTPSDeployKey struct {
		ID             int64
		RepoID         int64
		Name           string
		TokenHash      string
		TokenSalt      string
		TokenLastEight string
		Mode           int
		CreatedUnix    int64
		UpdatedUnix    int64
	}

	_, err := x.Insert(&HTTPSDeployKey{
		RepoID:         1,
		Name:           "migration-smoke",
		TokenHash:      "hash",
		TokenSalt:      "salt",
		TokenLastEight: "abcd1234",
		Mode:           1,
	})
	assert.NoError(t, err)

	count, err := x.Count(&HTTPSDeployKey{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
