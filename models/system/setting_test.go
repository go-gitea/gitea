// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/system"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test.key"
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, db.TruncateBeans(t.Context(), &system.Setting{}))

	rev, settings, err := system.GetAllSettings(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 1, rev)
	assert.Len(t, settings, 1) // there is only one "revision" key

	for index, value := range []string{"true", "false"} {
		err = system.SetSettings(t.Context(), map[string]string{keyName: value})
		assert.NoError(t, err)
		rev, settings, err = system.GetAllSettings(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, index+2, rev)
		assert.Len(t, settings, 2)
		assert.Equal(t, value, settings[keyName])
	}

	// setting the same value should not trigger DuplicateKey error, and the "version" should be increased
	err = system.SetSettings(t.Context(), map[string]string{keyName: "false"})
	assert.NoError(t, err)

	rev, settings, err = system.GetAllSettings(t.Context())
	assert.NoError(t, err)
	assert.Len(t, settings, 2)
	assert.Equal(t, 4, rev)
}
