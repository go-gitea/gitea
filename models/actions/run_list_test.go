// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetRunWorkflowIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := GetRunWorkflowIDs(t.Context(), 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"artifact.yaml", "test.yaml"}, ids)

	ids, err = GetRunWorkflowIDs(t.Context(), 999999)
	assert.NoError(t, err)
	assert.Empty(t, ids)
}
