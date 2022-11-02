// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_15 //nolint

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_AddPrimaryEmail2EmailAddress(t *testing.T) {
	type User struct {
		ID       int64
		Email    string
		IsActive bool
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(User))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	err := AddPrimaryEmail2EmailAddress(x)
	assert.NoError(t, err)

	type EmailAddress struct {
		ID          int64  `xorm:"pk autoincr"`
		UID         int64  `xorm:"INDEX NOT NULL"`
		Email       string `xorm:"UNIQUE NOT NULL"`
		LowerEmail  string `xorm:"UNIQUE NOT NULL"`
		IsActivated bool
		IsPrimary   bool `xorm:"DEFAULT(false) NOT NULL"`
	}

	users := make([]User, 0, 20)
	err = x.Find(&users)
	assert.NoError(t, err)

	for _, user := range users {
		var emailAddress EmailAddress
		has, err := x.Where("lower_email=?", strings.ToLower(user.Email)).Get(&emailAddress)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.True(t, emailAddress.IsPrimary)
		assert.EqualValues(t, user.IsActive, emailAddress.IsActivated)
		assert.EqualValues(t, user.ID, emailAddress.UID)
	}
}
