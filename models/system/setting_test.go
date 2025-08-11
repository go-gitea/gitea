// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test.key"
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, db.TruncateBeans(db.DefaultContext, &system.Setting{}))

	rev, settings, err := system.GetAllSettings(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, 1, rev)
	assert.Len(t, settings, 1) // there is only one "revision" key

	err = system.SetSettings(db.DefaultContext, map[string]string{keyName: "true"})
	assert.NoError(t, err)
	rev, settings, err = system.GetAllSettings(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, 2, rev)
	assert.Len(t, settings, 2)
	assert.Equal(t, "true", settings[keyName])

	err = system.SetSettings(db.DefaultContext, map[string]string{keyName: "false"})
	assert.NoError(t, err)
	rev, settings, err = system.GetAllSettings(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, 3, rev)
	assert.Len(t, settings, 2)
	assert.Equal(t, "false", settings[keyName])

	// setting the same value should not trigger DuplicateKey error, and the "version" should be increased
	err = system.SetSettings(db.DefaultContext, map[string]string{keyName: "false"})
	assert.NoError(t, err)

	rev, settings, err = system.GetAllSettings(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, settings, 2)
	assert.Equal(t, 4, rev)
}
