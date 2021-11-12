// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/gitea/models/unittest"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."))
}
