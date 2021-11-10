// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestMakeEmailPrimary(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	email := &user_model.EmailAddress{
		Email: "user567890@example.com",
	}
	err := MakeEmailPrimary(email)
	assert.Error(t, err)
	assert.EqualError(t, err, user_model.ErrEmailAddressNotExist{Email: email.Email}.Error())

	email = &user_model.EmailAddress{
		Email: "user11@example.com",
	}
	err = MakeEmailPrimary(email)
	assert.Error(t, err)
	assert.EqualError(t, err, user_model.ErrEmailNotActivated.Error())

	email = &user_model.EmailAddress{
		Email: "user9999999@example.com",
	}
	err = MakeEmailPrimary(email)
	assert.Error(t, err)
	assert.True(t, IsErrUserNotExist(err))

	email = &user_model.EmailAddress{
		Email: "user101@example.com",
	}
	err = MakeEmailPrimary(email)
	assert.NoError(t, err)

	user, _ := GetUserByID(int64(10))
	assert.Equal(t, "user101@example.com", user.Email)
}

func TestActivate(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	email := &user_model.EmailAddress{
		ID:    int64(1),
		UID:   int64(1),
		Email: "user11@example.com",
	}
	assert.NoError(t, ActivateEmail(email))

	emails, _ := user_model.GetEmailAddresses(int64(1))
	assert.Len(t, emails, 3)
	assert.True(t, emails[0].IsActivated)
	assert.True(t, emails[0].IsPrimary)
	assert.False(t, emails[1].IsPrimary)
	assert.True(t, emails[2].IsActivated)
	assert.False(t, emails[2].IsPrimary)
}

func TestListEmails(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	// Must find all users and their emails
	opts := &SearchEmailOptions{
		ListOptions: db.ListOptions{
			PageSize: 10000,
		},
	}
	emails, count, err := SearchEmails(opts)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), count)
	assert.True(t, count > 5)

	contains := func(match func(s *SearchEmailResult) bool) bool {
		for _, v := range emails {
			if match(v) {
				return true
			}
		}
		return false
	}

	assert.True(t, contains(func(s *SearchEmailResult) bool { return s.UID == 18 }))
	// 'user3' is an organization
	assert.False(t, contains(func(s *SearchEmailResult) bool { return s.UID == 3 }))

	// Must find no records
	opts = &SearchEmailOptions{Keyword: "NOTFOUND"}
	emails, count, err = SearchEmails(opts)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Must find users 'user2', 'user28', etc.
	opts = &SearchEmailOptions{Keyword: "user2"}
	emails, count, err = SearchEmails(opts)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), count)
	assert.True(t, contains(func(s *SearchEmailResult) bool { return s.UID == 2 }))
	assert.True(t, contains(func(s *SearchEmailResult) bool { return s.UID == 27 }))

	// Must find only primary addresses (i.e. from the `user` table)
	opts = &SearchEmailOptions{IsPrimary: util.OptionalBoolTrue}
	emails, count, err = SearchEmails(opts)
	assert.NoError(t, err)
	assert.True(t, contains(func(s *SearchEmailResult) bool { return s.IsPrimary }))
	assert.False(t, contains(func(s *SearchEmailResult) bool { return !s.IsPrimary }))

	// Must find only inactive addresses (i.e. not validated)
	opts = &SearchEmailOptions{IsActivated: util.OptionalBoolFalse}
	emails, count, err = SearchEmails(opts)
	assert.NoError(t, err)
	assert.True(t, contains(func(s *SearchEmailResult) bool { return !s.IsActivated }))
	assert.False(t, contains(func(s *SearchEmailResult) bool { return s.IsActivated }))

	// Must find more than one page, but retrieve only one
	opts = &SearchEmailOptions{
		ListOptions: db.ListOptions{
			PageSize: 5,
			Page:     1,
		},
	}
	emails, count, err = SearchEmails(opts)
	assert.NoError(t, err)
	assert.Len(t, emails, 5)
	assert.Greater(t, count, int64(len(emails)))
}
