// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"testing"

	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestApplyDefaultUserRepoPermission(t *testing.T) {
	perm := Permission{
		AccessMode: perm_model.AccessModeNone,
		UnitsMode:  map[unit.Type]perm_model.AccessMode{},
	}

	perm.Units = []*repo_model.RepoUnit{
		{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeNone},
	}
	applyDefaultUserRepoPermission(nil, &perm)
	assert.False(t, perm.CanRead(unit.TypeWiki))

	perm.Units = []*repo_model.RepoUnit{
		{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeRead},
	}
	applyDefaultUserRepoPermission(&user_model.User{ID: 1}, &perm)
	assert.True(t, perm.CanRead(unit.TypeWiki))
}
