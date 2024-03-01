// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	organization_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestAddOrSetPrimaryEmailAddress(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 27})

	emails, err := user_model.GetEmailAddresses(db.DefaultContext, user.ID)
	assert.NoError(t, err)
	assert.Len(t, emails, 1)

	primary, err := user_model.GetPrimaryEmailAddressOfUser(db.DefaultContext, user.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, "new-primary@example.com", primary.Email)
	assert.Equal(t, user.Email, primary.Email)

	assert.NoError(t, AddOrSetPrimaryEmailAddress(db.DefaultContext, user, "new-primary@example.com"))

	primary, err = user_model.GetPrimaryEmailAddressOfUser(db.DefaultContext, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "new-primary@example.com", primary.Email)
	assert.Equal(t, user.Email, primary.Email)

	emails, err = user_model.GetEmailAddresses(db.DefaultContext, user.ID)
	assert.NoError(t, err)
	assert.Len(t, emails, 2)

	assert.NoError(t, AddOrSetPrimaryEmailAddress(db.DefaultContext, user, "user27@example.com"))

	primary, err = user_model.GetPrimaryEmailAddressOfUser(db.DefaultContext, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "user27@example.com", primary.Email)
	assert.Equal(t, user.Email, primary.Email)

	emails, err = user_model.GetEmailAddresses(db.DefaultContext, user.ID)
	assert.NoError(t, err)
	assert.Len(t, emails, 2)
}

func TestReplacePrimaryEmailAddress(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("User", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})

		emails, err := user_model.GetEmailAddresses(db.DefaultContext, user.ID)
		assert.NoError(t, err)
		assert.Len(t, emails, 1)

		primary, err := user_model.GetPrimaryEmailAddressOfUser(db.DefaultContext, user.ID)
		assert.NoError(t, err)
		assert.NotEqual(t, "primary-13@example.com", primary.Email)
		assert.Equal(t, user.Email, primary.Email)

		assert.NoError(t, ReplacePrimaryEmailAddress(db.DefaultContext, user, "primary-13@example.com"))

		primary, err = user_model.GetPrimaryEmailAddressOfUser(db.DefaultContext, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, "primary-13@example.com", primary.Email)
		assert.Equal(t, user.Email, primary.Email)

		emails, err = user_model.GetEmailAddresses(db.DefaultContext, user.ID)
		assert.NoError(t, err)
		assert.Len(t, emails, 1)

		assert.NoError(t, ReplacePrimaryEmailAddress(db.DefaultContext, user, "primary-13@example.com"))
	})

	t.Run("Organization", func(t *testing.T) {
		org := unittest.AssertExistsAndLoadBean(t, &organization_model.Organization{ID: 3})

		assert.Equal(t, "org3@example.com", org.Email)

		assert.NoError(t, ReplacePrimaryEmailAddress(db.DefaultContext, org.AsUser(), "primary-org@example.com"))

		assert.Equal(t, "primary-org@example.com", org.Email)
	})
}

func TestAddEmailAddresses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.Error(t, AddEmailAddresses(db.DefaultContext, user, []string{" invalid email "}))

	emails := []string{"user1234@example.com", "user5678@example.com"}

	assert.NoError(t, AddEmailAddresses(db.DefaultContext, user, emails))

	err := AddEmailAddresses(db.DefaultContext, user, emails)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrEmailAlreadyUsed(err))
}

func TestDeleteEmailAddresses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	emails := []string{"user2-2@example.com"}

	err := DeleteEmailAddresses(db.DefaultContext, user, emails)
	assert.NoError(t, err)

	err = DeleteEmailAddresses(db.DefaultContext, user, emails)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrEmailAddressNotExist(err))

	emails = []string{"user2@example.com"}

	err = DeleteEmailAddresses(db.DefaultContext, user, emails)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrPrimaryEmailCannotDelete(err))
}
