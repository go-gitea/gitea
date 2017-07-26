package models

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3" // for the test engine
	"github.com/stretchr/testify/assert"
	"gopkg.in/testfixtures.v2"
)

// TestFixturesAreConsistent assert that test fixtures are consistent
func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	CheckConsistencyForAll(t)
}

// CreateTestEngine create an xorm engine for testing
func CreateTestEngine() error {
	var err error
	x, err = xorm.NewEngine("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		return err
	}
	x.SetMapper(core.GonicMapper{})
	if err = x.StoreEngine("InnoDB").Sync2(tables...); err != nil {
		return err
	}

	return InitFixtures(&testfixtures.SQLite{}, "fixtures/")
}

func TestMain(m *testing.M) {
	if err := CreateTestEngine(); err != nil {
		fmt.Printf("Error creating test engine: %v\n", err)
		os.Exit(1)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.RunUser = "runuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.RepoRootPath = filepath.Join(os.TempDir(), "repos")
	setting.AppDataPath = filepath.Join(os.TempDir(), "appdata")

	os.Exit(m.Run())
}
