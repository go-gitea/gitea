// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddStorageQuotas(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0)
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	t.Logf("database: %s", x.Dialect().URI().DBType)
	require.NoError(t, AddStorageQuotas(t.Context(), x))
	for _, name := range []string{"quota_policy", "quota_usage", "quota_operation"} {
		exists, err := x.IsTableExist(name)
		require.NoError(t, err)
		assert.True(t, exists, name)
	}
	type QuotaPolicy struct {
		ID        int64 `xorm:"pk autoincr"`
		Subject   string
		SubjectID int64
		GitLimit  *int64
		LFSLimit  *int64
	}
	_, err := x.Insert(&QuotaPolicy{Subject: "repository", SubjectID: 1, GitLimit: new(int64(0))})
	require.NoError(t, err)
	var policy QuotaPolicy
	has, err := x.Where("subject=? AND subject_id=?", "repository", 1).Get(&policy)
	require.NoError(t, err)
	require.True(t, has)
	require.NotNil(t, policy.GitLimit)
	assert.Zero(t, *policy.GitLimit)
	assert.Nil(t, policy.LFSLimit)
	_, err = x.Insert(&QuotaPolicy{Subject: "repository", SubjectID: 1})
	require.Error(t, err)
}
