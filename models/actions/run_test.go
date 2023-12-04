// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestFindWorkflowIDsByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := FindWorkflowIDsByRepoID(db.DefaultContext, 4)
	assert.NoError(t, err)

	assert.EqualValues(t, []string{"artifact.yaml"}, ids)
}
