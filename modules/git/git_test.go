// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func testRun(m *testing.M) error {
	gitHomePath, err := os.MkdirTemp(os.TempDir(), "git-home")
	if err != nil {
		return fmt.Errorf("unable to create temp dir: %w", err)
	}
	defer util.RemoveAll(gitHomePath)
	setting.Git.HomePath = gitHomePath

	if err = InitFull(context.Background()); err != nil {
		return fmt.Errorf("failed to call Init: %w", err)
	}

	exitCode := m.Run()
	if exitCode != 0 {
		return fmt.Errorf("run test failed, ExitCode=%d", exitCode)
	}
	return nil
}

func TestMain(m *testing.M) {
	if err := testRun(m); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Test failed: %v", err)
		os.Exit(1)
	}
}

func TestParseGitVersion(t *testing.T) {
	v, err := parseGitVersionLine("git version 2.29.3")
	assert.NoError(t, err)
	assert.Equal(t, "2.29.3", v.String())

	v, err = parseGitVersionLine("git version 2.29.3.windows.1")
	assert.NoError(t, err)
	assert.Equal(t, "2.29.3", v.String())

	_, err = parseGitVersionLine("git version")
	assert.Error(t, err)

	_, err = parseGitVersionLine("git version windows")
	assert.Error(t, err)
}

func TestCheckGitVersionCompatibility(t *testing.T) {
	assert.NoError(t, checkGitVersionCompatibility(version.Must(version.NewVersion("2.43.0"))))
	assert.ErrorContains(t, checkGitVersionCompatibility(version.Must(version.NewVersion("2.43.1"))), "regression bug of GIT_FLUSH")
	assert.NoError(t, checkGitVersionCompatibility(version.Must(version.NewVersion("2.43.2"))))
}
