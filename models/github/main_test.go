// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package github_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models/repo" // register Mirror model needed by CountMirrorsByCredentialID
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
