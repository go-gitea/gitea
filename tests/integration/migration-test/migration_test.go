// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations"
	migrate_base "code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

var currentEngine *xorm.Engine

func initMigrationTest(t *testing.T) func() {
	testlogger.Init()
	giteaRoot := test.SetupGiteaRoot()
	setting.AppPath = path.Join(giteaRoot, "gitea")
	if _, err := os.Stat(setting.AppPath); err != nil {
		testlogger.Fatalf(fmt.Sprintf("Could not find gitea binary at %s\n", setting.AppPath))
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		testlogger.Fatalf("Environment variable $GITEA_CONF not set\n")
	} else if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	unittest.InitSettings()

	assert.NotEmpty(t, setting.RepoRootPath)
	assert.NoError(t, unittest.SyncDirs(path.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))
	assert.NoError(t, git.InitFull(t.Context()))
	setting.LoadDBSetting()
	setting.InitLoggersForTest()

	return testlogger.PrintCurrentTest(t, 2)
}

func availableVersions() ([]string, error) {
	migrationsDir, err := os.Open("tests/integration/migration-test")
	if err != nil {
		return nil, err
	}
	defer migrationsDir.Close()
	versionRE, err := regexp.Compile("gitea-v(?P<version>.+)\\." + regexp.QuoteMeta(setting.Database.Type.String()) + "\\.sql.gz")
	if err != nil {
		return nil, err
	}

	filenames, err := migrationsDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	versions := []string{}
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

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", nil
	}

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

	bytes, err := io.ReadAll(gr)
	if err != nil {
		return "", err
	}
	return string(charset.MaybeRemoveBOM(bytes, charset.ConvertOpts{})), nil
}

func restoreOldDB(t *testing.T, version string) {
	data, err := readSQLFromFile(version)
	require.NoError(t, err)
	require.NotEmpty(t, data, "No data found for %s version: %s", setting.Database.Type, version)

	switch {
	case setting.Database.Type.IsSQLite3():
		util.Remove(setting.Database.Path)
		err := os.MkdirAll(path.Dir(setting.Database.Path), os.ModePerm)
		assert.NoError(t, err)

		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=%d&_txlock=immediate", setting.Database.Path, setting.Database.Timeout))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.Database.Type.IsMySQL():
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", setting.Database.Name))
		assert.NoError(t, err)

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", setting.Database.Name))
		assert.NoError(t, err)
		db.Close()

		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?multiStatements=true",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.Database.Type.IsPostgreSQL():
		var db *sql.DB
		var err error
		if setting.Database.Host[0] == '/' {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/?sslmode=%s&host=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.SSLMode, setting.Database.Host))
			assert.NoError(t, err)
		} else {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
			assert.NoError(t, err)
		}
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", setting.Database.Name))
		assert.NoError(t, err)

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", setting.Database.Name))
		assert.NoError(t, err)
		db.Close()

		// Check if we need to setup a specific schema
		if len(setting.Database.Schema) != 0 {
			if setting.Database.Host[0] == '/' {
				db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
					setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
			} else {
				db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
					setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
			}
			require.NoError(t, err)
			defer db.Close()

			schrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
			require.NoError(t, err)
			require.NotEmpty(t, schrows)

			if !schrows.Next() {
				// Create and setup a DB schema
				_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", setting.Database.Schema))
				assert.NoError(t, err)
			}
			schrows.Close()

			// Make the user's default search path the created schema; this will affect new connections
			_, err = db.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema))
			assert.NoError(t, err)

			db.Close()
		}

		if setting.Database.Host[0] == '/' {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
		} else {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		}
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.Database.Type.IsMSSQL():
		host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", setting.Database.User, setting.Database.Passwd))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec("DROP DATABASE IF EXISTS [gitea]")
		assert.NoError(t, err)

		statements := strings.Split(data, "\nGO\n")
		for _, statement := range statements {
			if len(statement) > 5 && statement[:5] == "USE [" {
				dbname := statement[5 : len(statement)-1]
				db.Close()
				db, err = sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
					host, port, dbname, setting.Database.User, setting.Database.Passwd))
				assert.NoError(t, err)
				defer db.Close()
			}
			_, err = db.Exec(statement)
			assert.NoError(t, err, "Failure whilst running: %s\nError: %v", statement, err)
		}
		db.Close()
	}
}

func wrappedMigrate(ctx context.Context, x *xorm.Engine) error {
	currentEngine = x
	return migrations.Migrate(x)
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
