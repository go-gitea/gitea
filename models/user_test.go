// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// ignore none active user email
	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user9"}))
	assert.Equal(t, []string{"user8@example.com", "user5@example.com"}, GetUserEmailsByNames([]string{"user8", "user5"}))
}
