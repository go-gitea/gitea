// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUserListIsPublicMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	//TODO
}

func TestUserListIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	//TODO
}

func TestUserListIsTwoFaEnrolled(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	//TODO
}
