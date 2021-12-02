// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestAccessLevel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29}).(*user_model.User)
	// A public repository owned by User 2
	repo1 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo3 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.True(t, repo3.IsPrivate)

	// Another public repository
	repo4 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.False(t, repo4.IsPrivate)
	// org. owned private repo
	repo24 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 24}).(*Repository)

	level, err := AccessLevel(user2, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, level)

	level, err = AccessLevel(user2, repo3)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, level)

	level, err = AccessLevel(user5, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, level)

	level, err = AccessLevel(user5, repo3)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, level)

	// restricted user has no access to a public repo
	level, err = AccessLevel(user29, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, level)

	// ... unless he's a collaborator
	level, err = AccessLevel(user29, repo4)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeWrite, level)

	// ... or a team member
	level, err = AccessLevel(user29, repo24)
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, level)
}

func TestHasAccess(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	// A public repository owned by User 2
	repo1 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo2 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.True(t, repo2.IsPrivate)

	has, err := HasAccess(user1.ID, repo1)
	assert.NoError(t, err)
	assert.True(t, has)

	_, err = HasAccess(user1.ID, repo2)
	assert.NoError(t, err)

	_, err = HasAccess(user2.ID, repo1)
	assert.NoError(t, err)

	_, err = HasAccess(user2.ID, repo2)
	assert.NoError(t, err)
}

func TestRepository_RecalculateAccesses(t *testing.T) {
	// test with organization repo
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.NoError(t, repo1.GetOwner())

	_, err := db.GetEngine(db.DefaultContext).Delete(&Collaboration{UserID: 2, RepoID: 3})
	assert.NoError(t, err)
	assert.NoError(t, repo1.RecalculateAccesses())

	access := &Access{UserID: 2, RepoID: 3}
	has, err := db.GetEngine(db.DefaultContext).Get(access)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, perm.AccessModeOwner, access.Mode)
}

func TestRepository_RecalculateAccesses2(t *testing.T) {
	// test with non-organization repo
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo1.GetOwner())

	_, err := db.GetEngine(db.DefaultContext).Delete(&Collaboration{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.NoError(t, repo1.RecalculateAccesses())

	has, err := db.GetEngine(db.DefaultContext).Get(&Access{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestRepository_RecalculateAccesses3(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	team5 := unittest.AssertExistsAndLoadBean(t, &Team{ID: 5}).(*Team)
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29}).(*user_model.User)

	has, err := db.GetEngine(db.DefaultContext).Get(&Access{UserID: 29, RepoID: 23})
	assert.NoError(t, err)
	assert.False(t, has)

	// adding user29 to team5 should add an explicit access row for repo 23
	// even though repo 23 is public
	assert.NoError(t, AddTeamMember(team5, user29.ID))

	has, err = db.GetEngine(db.DefaultContext).Get(&Access{UserID: 29, RepoID: 23})
	assert.NoError(t, err)
	assert.True(t, has)
}
