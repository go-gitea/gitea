// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"testing"

	"code.gitea.io/gitea/models"
)

// IMPORTANT: THIS FILE IS ONLY A BUILDING BLOCK TO HELP TEST THE FEATURE
// DURING DEVELOPMENT. IT'S NOT INTENDED TO GO LIKE THIS IN THE FINAL
// VERSION OF THE PR.

func TestUserRepoUnit(t *testing.T) {

	models.UserRepoUnitTest(t)
}
