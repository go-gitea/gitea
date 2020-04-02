// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/stretchr/testify/assert"
	"github.com/unknwon/com"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

func TestMain(m *testing.M) {
	giteaRoot := base.SetupGiteaRoot()
	if giteaRoot == "" {
		fmt.Println("Environment variable $GITEA_ROOT not set")
		os.Exit(1)
	}
	giteaBinary := "gitea"
	if runtime.GOOS == "windows" {
		giteaBinary += ".exe"
	}
	setting.AppPath = path.Join(giteaRoot, giteaBinary)
	if _, err := os.Stat(setting.AppPath); err != nil {
		fmt.Printf("Could not find gitea binary at %s\n", setting.AppPath)
		os.Exit(1)
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		giteaConf = path.Join(filepath.Dir(setting.AppPath), "integrations/sqlite.ini")
		fmt.Printf("Environment variable $GITEA_CONF not set - defaulting to %s\n", giteaConf)
	}

	if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	setting.SetCustomPathAndConf("", "", "")
	setting.NewContext()
	setting.CheckLFSVersion()
	setting.InitDBConfig()
	setting.NewLogServices(true)

	exitStatus := m.Run()

	if err := removeAllWithRetry(setting.RepoRootPath); err != nil {
		fmt.Fprintf(os.Stderr, "os.RemoveAll: %v\n", err)
	}
	if err := removeAllWithRetry(setting.AppDataPath); err != nil {
		fmt.Fprintf(os.Stderr, "os.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
}

func removeAllWithRetry(dir string) error {
	var err error
	for i := 0; i < 20; i++ {
		err = os.RemoveAll(dir)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func getEngine() (*xorm.Engine, error) {
	connStr, err := setting.DBConnStr()
	if err != nil {
		return nil, err
	}

	engine, err := xorm.NewEngine(setting.Database.Type, connStr)
	if err != nil {
		return nil, err
	}
	if setting.Database.Type == "mysql" {
		engine.Dialect().SetParams(map[string]string{"rowFormat": "DYNAMIC"})
	}
	engine.SetSchema(setting.Database.Schema)
	return engine, nil
}

// SetEngine sets the xorm.Engine
func SetEngine() (*xorm.Engine, error) {
	x, err := getEngine()
	if err != nil {
		return x, fmt.Errorf("Failed to connect to database: %v", err)
	}

	x.SetMapper(names.GonicMapper{})
	// WARNING: for serv command, MUST remove the output to os.stdout,
	// so use log file to instead print to stdout.
	x.SetLogger(models.NewXORMLogger(setting.Database.LogSQL))
	x.ShowSQL(setting.Database.LogSQL)
	x.SetMaxOpenConns(setting.Database.MaxOpenConns)
	x.SetMaxIdleConns(setting.Database.MaxIdleConns)
	x.SetConnMaxLifetime(setting.Database.ConnMaxLifetime)
	return x, nil
}

// prepareTestEnv prepares the test environment. The skip parameter should usually be 0. Provide models to be sync'd
// with the database - in particular any models you expect fixtures to be loaded from.
//
// fixtures in `models/migrations/fixtures/<TestName>` will be loaded automatically
func prepareTestEnv(t *testing.T, skip int, syncModels ...interface{}) (*xorm.Engine, func()) {
	t.Helper()
	ourSkip := 2
	ourSkip += skip
	deferFn := PrintCurrentTest(t, ourSkip)
	assert.NoError(t, os.RemoveAll(setting.RepoRootPath))

	assert.NoError(t, com.CopyDir(path.Join(filepath.Dir(setting.AppPath), "integrations/gitea-repositories-meta"),
		setting.RepoRootPath))

	x, err := SetEngine()
	assert.NoError(t, err)
	if x != nil {
		oldDefer := deferFn
		deferFn = func() {
			oldDefer()
			if err := x.Close(); err != nil {
				t.Errorf("error during close: %v", err)
			}
		}
	}
	if err != nil {
		return x, deferFn
	}

	if len(syncModels) > 0 {
		if err := x.Sync2(syncModels...); err != nil {
			t.Errorf("error during sync: %v", err)
			return x, deferFn
		}
	}

	fixturesDir := filepath.Join(filepath.Dir(setting.AppPath), "models", "migrations", "fixtures", t.Name())

	if _, err := os.Stat(fixturesDir); err == nil {
		t.Logf("initializing fixtures from: %s", fixturesDir)
		if err := models.InitFixtures(fixturesDir, x); err != nil {
			t.Errorf("error whilst initializing fixtures from %s: %v", fixturesDir, err)
			return x, deferFn
		}
		if err := models.LoadFixtures(x); err != nil {
			t.Errorf("error whilst loading fixtures from %s: %v", fixturesDir, err)
			return x, deferFn
		}
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error whilst checking for existence of fixtures: %v", err)
	} else {
		t.Logf("no fixtures found in: %s", fixturesDir)
	}

	return x, deferFn
}

func Test_dropTableColumns(t *testing.T) {
	x, deferable := prepareTestEnv(t, 0)
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	type DropTest struct {
		ID            int64 `xorm:"pk autoincr"`
		FirstColumn   string
		ToDropColumn  string `xorm:"unique"`
		AnotherColumn int64
		CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	columns := []string{
		"FirstColumn",
		"ToDropColumn",
		"AnotherColumn",
		"CreatedUnix",
		"UpdatedUnix",
	}

	for i := range columns {
		if err := x.Sync2(new(DropTest)); err != nil {
			assert.Fail(t, "%v", err)
			return
		}
		sess := x.NewSession()

		if err := dropTableColumns(sess, "drop_test", columns[i:]...); err != nil {
			sess.Close()
			assert.Fail(t, "%v", err)
			return
		}
		sess.Close()
		if _, err := x.DB().DB.Exec("DROP TABLE drop_test"); err != nil {
			assert.Fail(t, "%v", err)
			return
		}
		for j := range columns[i:] {
			if err := x.Sync2(new(DropTest)); err != nil {
				assert.Fail(t, "%v", err)
				return
			}
			dropcols := append([]string{columns[i]}, columns[j:]...)
			sess := x.NewSession()
			if err := dropTableColumns(sess, "drop_test", dropcols...); err != nil {
				sess.Close()
				assert.Fail(t, "%v", err)
				return
			}
			sess.Close()
			if _, err := x.DB().DB.Exec("DROP TABLE drop_test"); err != nil {
				assert.Fail(t, "%v", err)
				return
			}
		}
	}
}
