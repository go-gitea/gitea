// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base_test

import (
	"testing"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/require"
)

type legacyReview struct {
	ID   int64 `xorm:"pk autoincr"`
	Type string
}

func (legacyReview) TableName() string { return "review_legacy_cast" }

type currentReview struct {
	ID   int64 `xorm:"pk autoincr"`
	Type int
}

func (currentReview) TableName() string { return "review_legacy_cast" }

func TestRecreateTableCastsLegacyStringToInt(t *testing.T) {
	if !setting.Database.Type.IsPostgreSQL() {
		t.Skip("PostgreSQL-specific column cast")
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	require.NoError(t, x.Sync(new(legacyReview)))
	_, err := x.Insert(&legacyReview{Type: "2"})
	require.NoError(t, err)

	sess := x.NewSession()
	require.NoError(t, sess.Begin())
	require.NoError(t, base.RecreateTable(sess, new(currentReview)))
	require.NoError(t, sess.Commit())
	require.NoError(t, sess.Close())

	var got currentReview
	has, err := x.ID(1).Get(&got)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, 2, got.Type)
}
