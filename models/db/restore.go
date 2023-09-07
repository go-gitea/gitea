// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"

	"github.com/go-testfixtures/testfixtures/v3"
	"xorm.io/xorm/schemas"
)

func RestoreDatabase(dirPath string) error {
	var dialect string
	switch x.Dialect().URI().DBType {
	case schemas.POSTGRES:
		dialect = "postgres"
	case schemas.MYSQL:
		dialect = "mysql"
	case schemas.MSSQL:
		dialect = "mssql"
	case schemas.SQLITE:
		dialect = "sqlite3"
	default:
		return fmt.Errorf("unsupported RDBMS for integration tests")
	}

	loaderOptions := []func(loader *testfixtures.Loader) error{
		testfixtures.Database(x.DB().DB),
		testfixtures.Dialect(dialect),
		testfixtures.DangerousSkipTestDatabaseCheck(),
		testfixtures.Directory(dirPath),
	}

	if x.Dialect().URI().DBType == schemas.POSTGRES {
		loaderOptions = append(loaderOptions, testfixtures.SkipResetSequences())
	}

	fixtures, err := testfixtures.New(loaderOptions...)
	if err != nil {
		return err
	}

	return fixtures.Load()
}
