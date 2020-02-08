// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserRepoUnit(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, UserRepoUnitTestDo(x))
}
