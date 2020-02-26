// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/com"
	"gopkg.in/testfixtures.v2"
	"xorm.io/core"
	"xorm.io/xorm"
)

// NonexistentID an ID that will never exist
const NonexistentID = int64(math.MaxInt64)

// giteaRoot a path to the gitea root
var giteaRoot string

func fatalTestError(fmtStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

// MainTest a reusable TestMain(..) function for unit tests that need to use a
// test database. Creates the test database, and sets necessary settings.
func MainTest(m *testing.M, pathToGiteaRoot string) {
	var err error
	giteaRoot = pathToGiteaRoot
	fixturesDir := filepath.Join(pathToGiteaRoot, "models", "fixtures")
	if err = createTestEngine(fixturesDir); err != nil {
		fatalTestError("Error creating test engine: %v\n", err)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.RunUser = "runuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.Database.UseSQLite3 = true
	setting.RepoRootPath, err = ioutil.TempDir(os.TempDir(), "repos")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppDataPath, err = ioutil.TempDir(os.TempDir(), "appdata")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppWorkPath = pathToGiteaRoot
	setting.StaticRootPath = pathToGiteaRoot
	setting.GravatarSourceURL, err = url.Parse("https://secure.gravatar.com/avatar/")
	if err != nil {
		fatalTestError("url.Parse: %v\n", err)
	}

	if err = removeAllWithRetry(setting.RepoRootPath); err != nil {
		fatalTestError("os.RemoveAll: %v\n", err)
	}
	if err = com.CopyDir(filepath.Join(pathToGiteaRoot, "integrations", "gitea-repositories-meta"), setting.RepoRootPath); err != nil {
		fatalTestError("com.CopyDir: %v\n", err)
	}

	exitStatus := m.Run()
	if err = removeAllWithRetry(setting.RepoRootPath); err != nil {
		fatalTestError("os.RemoveAll: %v\n", err)
	}
	if err = removeAllWithRetry(setting.AppDataPath); err != nil {
		fatalTestError("os.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
}

func createTestEngine(fixturesDir string) error {
	var err error
	x, err = xorm.NewEngine("sqlite3", "file::memory:?cache=shared&_txlock=immediate")
	if err != nil {
		return err
	}
	x.SetMapper(core.GonicMapper{})
	if err = x.StoreEngine("InnoDB").Sync2(tables...); err != nil {
		return err
	}
	switch os.Getenv("GITEA_UNIT_TESTS_VERBOSE") {
	case "true", "1":
		x.ShowSQL(true)
	}

	return InitFixtures(&testfixtures.SQLite{}, fixturesDir)
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

// PrepareTestDatabase load test fixtures into test database
func PrepareTestDatabase() error {
	return LoadFixtures()
}

// PrepareTestEnv prepares the environment for unit tests. Can only be called
// by tests that use the above MainTest(..) function.
func PrepareTestEnv(t testing.TB) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, removeAllWithRetry(setting.RepoRootPath))
	metaPath := filepath.Join(giteaRoot, "integrations", "gitea-repositories-meta")
	assert.NoError(t, com.CopyDir(metaPath, setting.RepoRootPath))
	base.SetupGiteaRoot() // Makes sure GITEA_ROOT is set
}

type testCond struct {
	query interface{}
	args  []interface{}
}

// Cond create a condition with arguments for a test
func Cond(query interface{}, args ...interface{}) interface{} {
	return &testCond{query: query, args: args}
}

func whereConditions(sess *xorm.Session, conditions []interface{}) {
	for _, condition := range conditions {
		switch cond := condition.(type) {
		case *testCond:
			sess.Where(cond.query, cond.args...)
		default:
			sess.Where(cond)
		}
	}
}

func loadBeanIfExists(bean interface{}, conditions ...interface{}) (bool, error) {
	sess := x.NewSession()
	defer sess.Close()
	whereConditions(sess, conditions)
	return sess.Get(bean)
}

// BeanExists for testing, check if a bean exists
func BeanExists(t testing.TB, bean interface{}, conditions ...interface{}) bool {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	return exists
}

// AssertExistsAndLoadBean assert that a bean exists and load it from the test
// database
func AssertExistsAndLoadBean(t testing.TB, bean interface{}, conditions ...interface{}) interface{} {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.True(t, exists,
		"Expected to find %+v (of type %T, with conditions %+v), but did not",
		bean, bean, conditions)
	return bean
}

// GetCount get the count of a bean
func GetCount(t testing.TB, bean interface{}, conditions ...interface{}) int {
	sess := x.NewSession()
	defer sess.Close()
	whereConditions(sess, conditions)
	count, err := sess.Count(bean)
	assert.NoError(t, err)
	return int(count)
}

// AssertNotExistsBean assert that a bean does not exist in the test database
func AssertNotExistsBean(t testing.TB, bean interface{}, conditions ...interface{}) {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// AssertExistsIf asserts that a bean exists or does not exist, depending on
// what is expected.
func AssertExistsIf(t *testing.T, expected bool, bean interface{}, conditions ...interface{}) {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.Equal(t, expected, exists)
}

// AssertSuccessfulInsert assert that beans is successfully inserted
func AssertSuccessfulInsert(t testing.TB, beans ...interface{}) {
	_, err := x.Insert(beans...)
	assert.NoError(t, err)
}

// AssertCount assert the count of a bean
func AssertCount(t testing.TB, bean interface{}, expected interface{}) {
	assert.EqualValues(t, expected, GetCount(t, bean))
}

// AssertInt64InRange assert value is in range [low, high]
func AssertInt64InRange(t testing.TB, low, high, value int64) {
	assert.True(t, value >= low && value <= high,
		"Expected value in range [%d, %d], found %d", low, high, value)
}
