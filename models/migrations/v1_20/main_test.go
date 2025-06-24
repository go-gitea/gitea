// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
)

func TestMain(m *testing.M) {
	base.MainTest(m)
}
