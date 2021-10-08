// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// NonexistentID an ID that will never exist
const NonexistentID = int64(math.MaxInt64)

// giteaRoot a path to the gitea root
var (
	giteaRoot   string
	fixturesDir string
)

// FixturesDir returns the fixture directory
func FixturesDir() string {
	return fixturesDir
}

func fatalTestError(fmtStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

// MainTest a reusable TestMain(..) function for unit tests that need to use a
// test database. Creates the test database, and sets necessary settings.
func MainTest(m *testing.M, pathToGiteaRoot string, fixtureFiles ...string) {
	var err error
	giteaRoot = pathToGiteaRoot
	fixturesDir = filepath.Join(pathToGiteaRoot, "models", "fixtures")

	var opts FixturesOptions
	if len(fixtureFiles) == 0 {
		opts.Dir = fixturesDir
	} else {
		for _, f := range fixtureFiles {
			if len(f) != 0 {
				opts.Files = append(opts.Files, filepath.Join(fixturesDir, f))
			}
		}
	}

	if err = CreateTestEngine(opts); err != nil {
		fatalTestError("Error creating test engine: %v\n", err)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.RunUser = "runuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.Database.UseSQLite3 = true
	setting.RepoRootPath, err = os.MkdirTemp(os.TempDir(), "repos")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppDataPath, err = os.MkdirTemp(os.TempDir(), "appdata")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppWorkPath = pathToGiteaRoot
	setting.StaticRootPath = pathToGiteaRoot
	setting.GravatarSourceURL, err = url.Parse("https://secure.gravatar.com/avatar/")
	if err != nil {
		fatalTestError("url.Parse: %v\n", err)
	}
	setting.Attachment.Storage.Path = filepath.Join(setting.AppDataPath, "attachments")

	setting.LFS.Storage.Path = filepath.Join(setting.AppDataPath, "lfs")

	setting.Avatar.Storage.Path = filepath.Join(setting.AppDataPath, "avatars")

	setting.RepoAvatar.Storage.Path = filepath.Join(setting.AppDataPath, "repo-avatars")

	setting.RepoArchive.Storage.Path = filepath.Join(setting.AppDataPath, "repo-archive")

	if err = storage.Init(); err != nil {
		fatalTestError("storage.Init: %v\n", err)
	}

	if err = util.RemoveAll(setting.RepoRootPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	if err = util.CopyDir(filepath.Join(pathToGiteaRoot, "integrations", "gitea-repositories-meta"), setting.RepoRootPath); err != nil {
		fatalTestError("util.CopyDir: %v\n", err)
	}

	exitStatus := m.Run()
	if err = util.RemoveAll(setting.RepoRootPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	if err = util.RemoveAll(setting.AppDataPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
}

// FixturesOptions fixtures needs to be loaded options
type FixturesOptions struct {
	Dir   string
	Files []string
}

// CreateTestEngine creates a memory database and loads the fixture data from fixturesDir
func CreateTestEngine(opts FixturesOptions) error {
	var err error
	x, err = xorm.NewEngine("sqlite3", "file::memory:?cache=shared&_txlock=immediate")
	if err != nil {
		return err
	}
	x.SetMapper(names.GonicMapper{})
	if err = syncTables(); err != nil {
		return err
	}
	switch os.Getenv("GITEA_UNIT_TESTS_VERBOSE") {
	case "true", "1":
		x.ShowSQL(true)
	}

	DefaultContext = &Context{
		Context: context.Background(),
		e:       x,
	}

	return InitFixtures(opts)
}

// PrepareTestDatabase load test fixtures into test database
func PrepareTestDatabase() error {
	return LoadFixtures()
}

// PrepareTestEnv prepares the environment for unit tests. Can only be called
// by tests that use the above MainTest(..) function.
func PrepareTestEnv(t testing.TB) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, util.RemoveAll(setting.RepoRootPath))
	metaPath := filepath.Join(giteaRoot, "integrations", "gitea-repositories-meta")
	assert.NoError(t, util.CopyDir(metaPath, setting.RepoRootPath))
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

// LoadBeanIfExists loads beans from fixture database if exist
func LoadBeanIfExists(bean interface{}, conditions ...interface{}) (bool, error) {
	return loadBeanIfExists(bean, conditions...)
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
func AssertCount(t testing.TB, bean, expected interface{}) {
	assert.EqualValues(t, expected, GetCount(t, bean))
}

// AssertInt64InRange assert value is in range [low, high]
func AssertInt64InRange(t testing.TB, low, high, value int64) {
	assert.True(t, value >= low && value <= high,
		"Expected value in range [%d, %d], found %d", low, high, value)
}
