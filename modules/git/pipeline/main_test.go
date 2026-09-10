// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"

	"gitea.dev/modules/git"
)

func TestMain(m *testing.M) {
	git.RunGitTests(m)
}
