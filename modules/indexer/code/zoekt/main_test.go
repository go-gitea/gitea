// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build zoekt && unix

package zoekt

import (
	"fmt"
	"os"
	"testing"

	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
)

func TestMain(m *testing.M) {
	setting.Git.HomePath = os.TempDir()
	if err := git.InitFull(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to init git: %v", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
