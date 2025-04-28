// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_GetTeamsByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// 1 owner team, 2 normal team
	teams, err := org_model.GetTeamsByIDs(db.DefaultContext, []int64{1, 2})
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
	assert.Equal(t, "Owners", teams[1].Name)
	assert.Equal(t, "team1", teams[2].Name)
}
