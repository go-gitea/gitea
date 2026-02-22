// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package languagestats

import (
	"testing"

	"code.gitea.io/gitea/modules/git"
)

func TestMain(m *testing.M) {
	git.RunGitTests(m)
}
