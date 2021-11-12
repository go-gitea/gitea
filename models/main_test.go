// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

// TestFixturesAreConsistent assert that test fixtures are consistent
func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	CheckConsistencyFor(t,
		&User{},
		&Repository{},
		&Issue{},
		&PullRequest{},
		&Milestone{},
		&Label{},
		&Team{},
		&Action{})
}

func TestMain(m *testing.M) {
	unittest.MainTest(m, "..")
}
