// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_RegenerateSession(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	exist, err := auth.ExistSession(db.DefaultContext, "new_key")
	assert.NoError(t, err)
	assert.False(t, exist)

	sess, err := auth.RegenerateSession(db.DefaultContext, "", "new_key")
	assert.NoError(t, err)
	assert.EqualValues(t, "new_key", sess.Key)

	sess, err = auth.ReadSession(db.DefaultContext, "new_key2")
	assert.NoError(t, err)
	assert.EqualValues(t, "new_key2", sess.Key)
}
