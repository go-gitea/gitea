// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/migrations/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/require"
)

type externalLoginUserBeforeDateTimeMigration struct {
	ExternalID    string    `xorm:"pk NOT NULL"`
	LoginSourceID int64     `xorm:"pk NOT NULL"`
	ExpiresAt     time.Time // sync creates DATETIME2; downgraded to legacy DATETIME via raw SQL below
}

func (externalLoginUserBeforeDateTimeMigration) TableName() string {
	return "external_login_user"
}

type lfsLockBeforeDateTimeMigration struct {
	ID      int64     `xorm:"pk autoincr"`
	Created time.Time `xorm:"created"`
}

func (lfsLockBeforeDateTimeMigration) TableName() string {
	return "lfs_lock"
}

func Test_FixLegacyMSSQLDateTimeColumns(t *testing.T) {
	if !setting.Database.Type.IsMSSQL() {
		t.Skip("Only MSSQL needs to convert the legacy locale-dependent DATETIME columns")
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0,
		new(externalLoginUserBeforeDateTimeMigration),
		new(lfsLockBeforeDateTimeMigration),
	)
	defer deferrable()

	// Force the legacy DATETIME column type that old Gitea versions created.
	_, err := x.Exec("ALTER TABLE [external_login_user] ALTER COLUMN [expires_at] DATETIME")
	require.NoError(t, err)
	_, err = x.Exec("ALTER TABLE [lfs_lock] ALTER COLUMN [created] DATETIME")
	require.NoError(t, err)
	require.Equal(t, "datetime", mssqlColumnType(t, x, "external_login_user", "expires_at"))
	require.Equal(t, "datetime", mssqlColumnType(t, x, "lfs_lock", "created"))

	require.NoError(t, FixLegacyMSSQLDateTimeColumns(x))
	require.NoError(t, FixLegacyMSSQLDateTimeColumns(x)) // idempotent

	require.Equal(t, "datetime2", mssqlColumnType(t, x, "external_login_user", "expires_at"))
	require.Equal(t, "datetime2", mssqlColumnType(t, x, "lfs_lock", "created"))

	// Inserting an ISO-formatted datetime must succeed even under a non-English
	// locale, which is the failure the legacy DATETIME columns produced. The
	// SET LANGUAGE and INSERT run in one Exec so they share a single connection.
	_, err = x.Exec("SET LANGUAGE German; " +
		"INSERT INTO [external_login_user] ([external_id], [login_source_id], [expires_at]) " +
		"VALUES ('ext-id', 1, '2026-06-25 11:58:39')")
	require.NoError(t, err)
	_, err = x.Exec("SET LANGUAGE German; " +
		"INSERT INTO [lfs_lock] ([created]) VALUES ('2026-06-25 11:58:39')")
	require.NoError(t, err)
}

func mssqlColumnType(t *testing.T, x db.EngineMigration, table, column string) string {
	t.Helper()
	var dataType string
	has, err := x.SQL("SELECT DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?", table, column).Get(&dataType)
	require.NoError(t, err)
	require.True(t, has)
	return dataType
}
