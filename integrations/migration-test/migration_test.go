// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"compress/gzip"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/xorm"
	"github.com/stretchr/testify/assert"
)

var currentEngine *xorm.Engine

func initMigrationTest() {
	giteaRoot := os.Getenv("GITEA_ROOT")
	if giteaRoot == "" {
		fmt.Println("Environment variable $GITEA_ROOT not set")
		os.Exit(1)
	}
	setting.AppPath = path.Join(giteaRoot, "gitea")
	if _, err := os.Stat(setting.AppPath); err != nil {
		fmt.Printf("Could not find gitea binary at %s\n", setting.AppPath)
		os.Exit(1)
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		fmt.Println("Environment variable $GITEA_CONF not set")
		os.Exit(1)
	} else if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	setting.NewContext()
	setting.CheckLFSVersion()
	models.LoadConfigs()

}

func readSQLFromFile(dialect, version string) (string, error) {
	filename := fmt.Sprintf("integrations/migration-test/gitea-v%s.%s.sql.gz", version, dialect)

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

	bytes, err := ioutil.ReadAll(gr)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func restoreOldDB(t *testing.T, version string) bool {
	dialect := "sqlite"
	switch {
	case setting.UseSQLite3:
		dialect = "sqlite"
	case setting.UseMySQL:
		dialect = "mysql"
	case setting.UsePostgreSQL:
		dialect = "pgsql"
	case setting.UseMSSQL:
		dialect = "mssql"
	}
	log.Printf("Attempting to restore old DB for %s version: %s\n", dialect, version)

	data, err := readSQLFromFile(dialect, version)
	assert.NoError(t, err)
	if len(data) == 0 {
		log.Printf("No db found to restore for %s version: %s\n", dialect, version)
		return false
	}

	switch {
	case setting.UseSQLite3:
		os.Remove(models.DbCfg.Path)
		err := os.MkdirAll(path.Dir(models.DbCfg.Path), os.ModePerm)
		assert.NoError(t, err)

		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=%d", models.DbCfg.Path, models.DbCfg.Timeout))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.UseMySQL:
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			models.DbCfg.User, models.DbCfg.Passwd, models.DbCfg.Host))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", models.DbCfg.Name))
		assert.NoError(t, err)

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", models.DbCfg.Name))
		assert.NoError(t, err)

		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?multiStatements=true",
			models.DbCfg.User, models.DbCfg.Passwd, models.DbCfg.Host, models.DbCfg.Name))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.UsePostgreSQL:
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			models.DbCfg.User, models.DbCfg.Passwd, models.DbCfg.Host, models.DbCfg.SSLMode))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", models.DbCfg.Name))
		assert.NoError(t, err)

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", models.DbCfg.Name))
		assert.NoError(t, err)
		db.Close()

		db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			models.DbCfg.User, models.DbCfg.Passwd, models.DbCfg.Host, models.DbCfg.Name, models.DbCfg.SSLMode))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.UseMSSQL:
		host, port := models.ParseMSSQLHostPort(models.DbCfg.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", models.DbCfg.User, models.DbCfg.Passwd))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec("DROP DATABASE IF EXISTS gitea")
		assert.NoError(t, err)

		_, err = db.Exec("CREATE DATABASE gitea")
		assert.NoError(t, err)

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()
	}
	return true
}

func doMigrate(x *xorm.Engine) error {
	currentEngine = x
	return migrations.Migrate(x)
}

func doMigrationTest(t *testing.T, version string) {
	initMigrationTest()
	if !restoreOldDB(t, version) {
		return
	}

	setting.NewXORMLogService(false)
	err := models.SetEngine()
	assert.NoError(t, err)

	err = models.NewEngine(doMigrate)
	assert.NoError(t, err)
	currentEngine.Close()
}

func TestMigrationV1_5_3(t *testing.T) {
	doMigrationTest(t, "1.5.3")
}

func TestMigrationV1_6_4(t *testing.T) {
	doMigrationTest(t, "1.6.4")
}

func TestMigrationV1_7_0(t *testing.T) {
	doMigrationTest(t, "1.7.0")
}
