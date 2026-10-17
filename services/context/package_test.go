// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

var (
	publicRepo  = &repo_model.Repository{IsPrivate: false}
	privateRepo = &repo_model.Repository{IsPrivate: true}
)

func TestDeterminePackageAccessModeForLimitedOwner(t *testing.T) {
	owner := &user.User{ID: 1, Visibility: structs.VisibleTypeLimited}

	accessMode, err := determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Doer: &user.User{ID: 2, IsActive: true}})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Doer: &user.User{ID: 3, IsActive: true, IsRestricted: true}})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, accessMode)
}

func TestDeterminePackageAccessModeForRepoVisibility(t *testing.T) {
	owner := &user.User{ID: 100, IsActive: true, Visibility: structs.VisibleTypePublic}
	doer := &user.User{ID: 2, IsActive: true}

	// package attached to a public repository stays readable
	accessMode, err := determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Doer: doer, Repository: publicRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	// package attached to a private repository is not
	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Doer: doer, Repository: privateRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, accessMode)

	// package without a repository keeps the previous behaviour
	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Doer: doer})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	// an anonymous doer is subject to the same repository check
	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Repository: publicRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Repository: privateRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, accessMode)

	// the package owner is never restricted by repository visibility
	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: owner, Doer: owner, Repository: privateRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, accessMode)
}

func TestDeterminePackageAccessModeForOrgRepoVisibility(t *testing.T) {
	// a logged-in doer needs DB-backed org/team lookups, so only the anonymous path is covered here
	org := &user.User{ID: 200, IsActive: true, Type: user.UserTypeOrganization, Visibility: structs.VisibleTypePublic}

	accessMode, err := determineAccessMode(&packageAssignmentCtx{ContextUser: org, Repository: publicRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: org, Repository: privateRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, accessMode)

	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: org})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	// a private organization is invisible, regardless of repository visibility
	privateOrg := &user.User{ID: 201, IsActive: true, Type: user.UserTypeOrganization, Visibility: structs.VisibleTypePrivate}
	accessMode, err = determineAccessMode(&packageAssignmentCtx{ContextUser: privateOrg, Repository: publicRepo})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, accessMode)
}
