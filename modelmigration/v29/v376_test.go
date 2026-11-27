// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddActionEnvironmentSchema(t *testing.T) {
	type Secret struct {
		ID          int64              `xorm:"pk autoincr"`
		OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
		RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data        string             `xorm:"LONGTEXT"`
		Description string             `xorm:"TEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	}
	type ActionVariable struct {
		ID          int64              `xorm:"pk autoincr"`
		OwnerID     int64              `xorm:"UNIQUE(owner_repo_name)"`
		RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
		Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data        string             `xorm:"LONGTEXT NOT NULL"`
		Description string             `xorm:"TEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(Secret), new(ActionVariable))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&Secret{RepoID: 1, Name: "KEY", Data: "kept-data", Description: "kept-description"},
		&ActionVariable{RepoID: 1, Name: "KEY", Data: "kept-data", Description: "kept-description"},
	)
	require.NoError(t, err)

	require.NoError(t, AddActionEnvironmentSchema(t.Context(), x))

	for _, table := range []string{"secret", "action_variable"} {
		var row struct {
			EnvironmentID int64
			Data          string
			Description   string
		}
		has, err := x.Table(table).Get(&row)
		require.NoError(t, err)
		require.True(t, has)
		assert.Zero(t, row.EnvironmentID)
		assert.Equal(t, "kept-data", row.Data)
		assert.Equal(t, "kept-description", row.Description)

		sameNameInEnvironment := map[string]any{"owner_id": 0, "repo_id": 1, "environment_id": 2, "name": "KEY", "data": "", "created_unix": 1}
		_, err = x.Table(table).Insert(sameNameInEnvironment)
		require.NoError(t, err, "%s must allow a name again inside an environment", table)
		_, err = x.Table(table).Insert(sameNameInEnvironment)
		require.Error(t, err, "%s must keep names unique within an environment", table)
	}
}
