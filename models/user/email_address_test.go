// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
)

func TestGetEmailAddresses(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	emails, _ := GetEmailAddresses(int64(1))
	if assert.Len(t, emails, 3) {
		assert.True(t, emails[0].IsPrimary)
		assert.True(t, emails[2].IsActivated)
		assert.False(t, emails[2].IsPrimary)
	}

	emails, _ = GetEmailAddresses(int64(2))
	if assert.Len(t, emails, 2) {
		assert.True(t, emails[0].IsPrimary)
		assert.True(t, emails[0].IsActivated)
	}
}

func TestIsEmailUsed(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	isExist, _ := IsEmailUsed(db.DefaultContext, "")
	assert.True(t, isExist)
	isExist, _ = IsEmailUsed(db.DefaultContext, "user11@example.com")
	assert.True(t, isExist)
	isExist, _ = IsEmailUsed(db.DefaultContext, "user1234567890@example.com")
	assert.False(t, isExist)
}

func TestAddEmailAddress(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	assert.NoError(t, AddEmailAddress(&EmailAddress{
		Email:       "user1234567890@example.com",
		LowerEmail:  "user1234567890@example.com",
		IsPrimary:   true,
		IsActivated: true,
	}))

	// ErrEmailAlreadyUsed
	err := AddEmailAddress(&EmailAddress{
		Email:      "user1234567890@example.com",
		LowerEmail: "user1234567890@example.com",
	})
	assert.Error(t, err)
	assert.True(t, IsErrEmailAlreadyUsed(err))
}

func TestAddEmailAddresses(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	// insert multiple email address
	emails := make([]*EmailAddress, 2)
	emails[0] = &EmailAddress{
		Email:       "user1234@example.com",
		LowerEmail:  "user1234@example.com",
		IsActivated: true,
	}
	emails[1] = &EmailAddress{
		Email:       "user5678@example.com",
		LowerEmail:  "user5678@example.com",
		IsActivated: true,
	}
	assert.NoError(t, AddEmailAddresses(emails))

	// ErrEmailAlreadyUsed
	err := AddEmailAddresses(emails)
	assert.Error(t, err)
	assert.True(t, IsErrEmailAlreadyUsed(err))
}

func TestDeleteEmailAddress(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	assert.NoError(t, DeleteEmailAddress(&EmailAddress{
		UID:        int64(1),
		ID:         int64(33),
		Email:      "user1-2@example.com",
		LowerEmail: "user1-2@example.com",
	}))

	assert.NoError(t, DeleteEmailAddress(&EmailAddress{
		UID:        int64(1),
		Email:      "user1-3@example.com",
		LowerEmail: "user1-3@example.com",
	}))

	// Email address does not exist
	err := DeleteEmailAddress(&EmailAddress{
		UID:        int64(1),
		Email:      "user1234567890@example.com",
		LowerEmail: "user1234567890@example.com",
	})
	assert.Error(t, err)
}

func TestDeleteEmailAddresses(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	// delete multiple email address
	emails := make([]*EmailAddress, 2)
	emails[0] = &EmailAddress{
		UID:        int64(2),
		ID:         int64(3),
		Email:      "user2@example.com",
		LowerEmail: "user2@example.com",
	}
	emails[1] = &EmailAddress{
		UID:        int64(2),
		Email:      "user2-2@example.com",
		LowerEmail: "user2-2@example.com",
	}
	assert.NoError(t, DeleteEmailAddresses(emails))

	// ErrEmailAlreadyUsed
	err := DeleteEmailAddresses(emails)
	assert.Error(t, err)
}
