// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"testing"

	"code.gitea.io/gitea/models"
)

func TestIssueNoDupIndex(t *testing.T) {
	prepareTestEnv(t)

	models.TestIssueNoDupIndex(t)
}
