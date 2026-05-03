// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations"
	migrate_base "code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/testlogger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

var currentEngine *xorm.Engine

func initMigrationTest(t *testing.T) func() {
	testlogger.Init()
	require.NoError(t, setting.PrepareIntegrationTestConfig())
	setting.SetupGiteaTestEnv()

	assert.NotEmpty(t, setting.RepoRootPath)
	assert.NoError(t, unittest.SyncDirs(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/gitea-repositories-meta"), setting.RepoRootPath))
	assert.NoError(t, git.InitFull())
	setting.LoadDBSetting()
	setting.InitLoggersForTest()

	return testlogger.PrintCurrentTest(t, 2)
}

func availableVersions() ([]string, error) {
	migrationsDir, err := os.Open(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/integration/migration-test"))
	if err != nil {
		return nil, err
	}
	defer migrationsDir.Close()
	versionRE, err := regexp.Compile("gitea-v(?P<version>.+)" + regexp.QuoteMeta("."+string(setting.Database.Type)+".sql.gz"))
	if err != nil {
		return nil, err
	}

	filenames, err := migrationsDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, filename := range filenames {
		if versionRE.MatchString(filename) {
			substrings := versionRE.FindStringSubmatch(filename)
			versions = append(versions, substrings[1])
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func readSQLFromFile(version string) (string, error) {
	filename := fmt.Sprintf("tests/integration/migration-test/gitea-v%s.%s.sql.gz", version, setting.Database.Type)
	filename = filepath.Join(setting.GetGiteaTestSourceRoot(), filename)

	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	buf, err := io.ReadAll(gr)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimPrefix(buf, []byte("\xef\xbb\xbf"))), nil
}

func restoreOldDB(t *testing.T, version string) {
	data, err := readSQLFromFile(version)
	require.NoError(t, err)
	require.NotEmpty(t, data, "No data found for %s version: %s", setting.Database.Type, version)

	cleanup, err := unittest.ResetTestDatabase()
	require.NoError(t, err)
	_ = cleanup // no clean up yet (not needed at the moment)

	connOpts := db.GlobalConnOptions()

	if !connOpts.Type.IsMSSQL() {
		if connOpts.Type.IsMySQL() {
			connOpts.Database += "?multiStatements=true"
		}
		driver, connStr, err := db.ConnStr(connOpts)
		require.NoError(t, err)

		sqlDB, err := sql.Open(driver, connStr)
		require.NoError(t, err)
		defer sqlDB.Close()

		_, err = sqlDB.Exec(data)
		require.NoError(t, err)
		return
	}

	// MSSQL is special. the test fixture will create the [testgitea] database again, so drop it ahead if it exists
	driver, connStr, err := db.ConnStrDefaultDatabase(connOpts)
	require.NoError(t, err)
	sqlDB, err := sql.Open(driver, connStr)
	require.NoError(t, err)

	_, err = sqlDB.Exec("DROP DATABASE IF EXISTS [testgitea]")
	require.NoError(t, err, "drop existing database testgitea")

	for statement := range strings.SplitSeq(data, "\nGO\n") {
		if useStmtAfter, ok := strings.CutPrefix(statement, "USE ["); ok {
			_ = sqlDB.Close()
			dbname := strings.TrimSuffix(useStmtAfter, "]") // extract the database name from "USE [dbname]"
			connOpts.Database = dbname
			driver, connStr, err := db.ConnStr(connOpts)
			require.NoError(t, err)
			sqlDB, err = sql.Open(driver, connStr)
			require.NoError(t, err)
		}
		_, err = sqlDB.Exec(statement)
		require.NoError(t, err, "SQL Exec failed when running: %s\nError: %v", statement, err)
	}
	_ = sqlDB.Close()
}

func wrappedMigrate(ctx context.Context, x *xorm.Engine) error {
	currentEngine = x
	return migrations.Migrate(ctx, x)
}

func doMigrationTest(t *testing.T, version string) {
	defer testlogger.PrintCurrentTest(t)()
	restoreOldDB(t, version)

	setting.InitSQLLoggersForCli(log.INFO)

	err := db.InitEngineWithMigration(t.Context(), wrappedMigrate)
	assert.NoError(t, err)
	currentEngine.Close()

	beans, _ := db.NamesToBean()

	err = db.InitEngineWithMigration(t.Context(), func(ctx context.Context, x *xorm.Engine) error {
		currentEngine = x
		return migrate_base.RecreateTables(beans...)(x)
	})
	assert.NoError(t, err)
	currentEngine.Close()

	// We do this a second time to ensure that there is not a problem with retained indices
	err = db.InitEngineWithMigration(t.Context(), func(ctx context.Context, x *xorm.Engine) error {
		currentEngine = x
		return migrate_base.RecreateTables(beans...)(x)
	})
	assert.NoError(t, err)

	currentEngine.Close()
}

func TestMigrations(t *testing.T) {
	defer initMigrationTest(t)()

	dialect := setting.Database.Type
	versions, err := availableVersions()
	require.NoError(t, err)
	require.NotEmpty(t, versions, "No old database versions available to migration test for %s", dialect)

	for _, version := range versions {
		t.Run(fmt.Sprintf("Migrate-%s-%s", dialect, version), func(t *testing.T) {
			doMigrationTest(t, version)
		})
	}
}
