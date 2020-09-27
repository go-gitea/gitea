// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"
	"github.com/stretchr/testify/assert"
)

func TestUser_ToUser(t *testing.T) {

	user, err := models.GetUserByID(1)
	assert.NoError(t, err)
	assert.True(t, user.IsAdmin)

	apiUser := ToUser(user, true, true)
	assert.True(t, apiUser.IsAdmin)

	user, err = models.GetUserByID(2)
	assert.NoError(t, err)
	assert.False(t, user.IsAdmin)

	apiUser = ToUser(user, true, true)
	assert.False(t, apiUser.IsAdmin)
}
