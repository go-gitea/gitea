// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_17 // nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
)

func TestMain(m *testing.M) {
	base.MainTest(m)
}
