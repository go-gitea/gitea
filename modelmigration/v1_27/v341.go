// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"fmt"
	"strings"
	"time"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

// legacyDateTimeColumns are the persisted real datetime columns that old Gitea
// versions created as MSSQL DATETIME. Every other time value is stored as a
// unix timestamp integer, so these are the only columns affected.
var legacyDateTimeColumns = []struct {
	bean   any
	column string
}{
	{new(externalLoginUserWithExpiresAt), "expires_at"},
	{new(lfsLockWithCreated), "created"},
}

type externalLoginUserWithExpiresAt struct {
	ExpiresAt time.Time
}

func (externalLoginUserWithExpiresAt) TableName() string {
	return "external_login_user"
}

type lfsLockWithCreated struct {
	Created time.Time `xorm:"created"`
}

func (lfsLockWithCreated) TableName() string {
	return "lfs_lock"
}

// FixLegacyMSSQLDateTimeColumns converts legacy locale-dependent DATETIME columns
// to DATETIME2. Databases created by old Gitea versions stored these columns as
// DATETIME, which fails to parse ISO datetime strings ('YYYY-MM-DD HH:MM:SS')
// when the MSSQL session language is not English, breaking external account
// linking and LFS lock creation. New installs already use DATETIME2, so only
// legacy MSSQL columns need converting.
func FixLegacyMSSQLDateTimeColumns(x base.EngineMigration) error {
	if x.Dialect().URI().DBType != schemas.MSSQL {
		return nil
	}

	for _, c := range legacyDateTimeColumns {
		table, err := x.TableInfo(c.bean)
		if err != nil {
			return err
		}

		var dataType string
		has, err := x.SQL("SELECT DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?", table.Name, c.column).Get(&dataType)
		if err != nil {
			return err
		}
		if !has || !strings.EqualFold(dataType, "datetime") {
			continue
		}

		column := table.GetColumn(c.column)
		if column == nil {
			return fmt.Errorf("column %s does not exist in table %s", c.column, table.Name)
		}
		if err := base.ModifyColumn(x, table.Name, column); err != nil {
			return fmt.Errorf("modify %s.%s: %w", table.Name, c.column, err)
		}
	}

	return nil
}
