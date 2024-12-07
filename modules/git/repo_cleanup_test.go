// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"github.com/stretchr/testify/assert"
)

// This test mimics a repository having dangling locks. If the locks are older than the threshold, they should be
// removed. Otherwise, they'll remain and the command will fail.

func TestMaintainExistentLock(t *testing.T) {
	if runtime.GOOS != "linux" {
		// Need to use touch to change the last access time of the lock files
		t.Skip("Skipping test on non-linux OS")
	}

	shouldRemainLocked := func(lockFiles []string, err error) {
		assert.Error(t, err)
		for _, lockFile := range lockFiles {
			assert.FileExists(t, lockFile)
		}
	}

	shouldBeUnlocked := func(lockFiles []string, err error) {
		assert.NoError(t, err)
		for _, lockFile := range lockFiles {
			assert.NoFileExists(t, lockFile)
		}
	}

	t.Run("2 days lock file (1 hour threshold)", func(t *testing.T) {
		doTestLockCleanup(t, "2 days", time.Hour, shouldBeUnlocked)
	})

	t.Run("1 hour lock file (1 hour threshold)", func(t *testing.T) {
		doTestLockCleanup(t, "1 hour", time.Hour, shouldBeUnlocked)
	})

	t.Run("1 minutes lock file (1 hour threshold)", func(t *testing.T) {
		doTestLockCleanup(t, "1 minutes", time.Hour, shouldRemainLocked)
	})

	t.Run("1 hour lock file (2 hour threshold)", func(t *testing.T) {
		doTestLockCleanup(t, "1 hour", 2*time.Hour, shouldRemainLocked)
	})
}

func doTestLockCleanup(t *testing.T, lockAge string, threshold time.Duration, expectedResult func(lockFiles []string, err error)) {
	defer test.MockVariableValue(&setting.Repository, setting.Repository)()

	setting.Repository.DanglingLockThreshold = threshold

	if tmpDir, err := os.MkdirTemp("", "cleanup-after-crash"); err != nil {
		t.Fatal(err)
	} else {
		defer os.RemoveAll(tmpDir)

		if err := os.CopyFS(tmpDir, os.DirFS("../../tests/gitea-repositories-meta/org3/repo3.git")); err != nil {
			t.Fatal(err)
		}

		lockFiles := lockFilesFor(tmpDir)

		os.MkdirAll(tmpDir+"/objects/info/commit-graphs", os.ModeSticky|os.ModePerm)

		for _, lockFile := range lockFiles {
			createLockFiles(t, lockFile, lockAge)
		}

		cmd := NewCommand(context.Background(), "fetch")
		_, _, cmdErr := cmd.RunStdString(&RunOpts{Dir: tmpDir})

		expectedResult(lockFiles, cmdErr)
	}
}

func lockFilesFor(path string) []string {
	return []string{
		path + "/config.lock",
		path + "/HEAD.lock",
		path + "/objects/info/commit-graphs/commit-graph-chain.lock",
	}
}

func createLockFiles(t *testing.T, file, lockAge string) {
	cmd := exec.Command("touch", "-m", "-a", "-d", "-"+lockAge, file)
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
}
