package models

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

// TestFixturesAreConsistent assert that test fixtures are consistent
func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	CheckConsistencyForAll(t)
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
