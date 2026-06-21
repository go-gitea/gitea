// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"testing"

	"gitea.dev/models/migrations/migrationtest"
)

func TestMain(m *testing.M) {
	migrationtest.MainTest(m)
}
