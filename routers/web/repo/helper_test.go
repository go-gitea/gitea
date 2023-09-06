// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestMakeSelfOnTop(t *testing.T) {
	users := MakeSelfOnTop(nil, []*user.User{{ID: 2}, {ID: 1}})
	assert.Len(t, users, 2)
	assert.EqualValues(t, 2, users[0].ID)

	users = MakeSelfOnTop(&user.User{ID: 1}, []*user.User{{ID: 2}, {ID: 1}})
	assert.Len(t, users, 2)
	assert.EqualValues(t, 1, users[0].ID)

	users = MakeSelfOnTop(&user.User{ID: 2}, []*user.User{{ID: 2}, {ID: 1}})
	assert.Len(t, users, 2)
	assert.EqualValues(t, 2, users[0].ID)
}
