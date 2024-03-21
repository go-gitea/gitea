// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"testing"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func Test_checkTreePathProtected(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	pb := unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{ID: 1})

	kases := []struct {
		TreePath string
		CanPush  bool
	}{
		{
			TreePath: "allow_push",
			CanPush:  true,
		},
		{
			TreePath: "disallow_push",
			CanPush:  false,
		},
	}

	for _, kase := range kases {
		err := checkTreePathProtected(context.Background(), pb, user2, []string{kase.TreePath})
		if kase.CanPush {
			assert.Nil(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}
