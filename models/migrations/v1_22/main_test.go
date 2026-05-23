// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/migrationtest"
)

func TestMain(m *testing.M) {
	migrationtest.MainTest(m)
}
