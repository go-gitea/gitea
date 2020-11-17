// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}
