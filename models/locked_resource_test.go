// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

func TestLockedResource(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	withSession := func(t *testing.T, f func(t *testing.T, sess *xorm.Session) bool) {
		sess := x.NewSession()
		defer sess.Close()
		err := sess.Begin()
		if !assert.NoError(t, err) {
			return
		}
		if success := f(t, sess); !success {
			return
		}
		err = sess.Commit()
		assert.NoError(t, err)
	}

	// Get lock, increment counter value
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		lck1, err := GetLockedResource(sess, "test-1", 1)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, lck1) || !assert.Equal(t, int64(0), lck1.Counter) {
			return false
		}
		lck1.Counter++
		err = UpdateLockedResource(sess, lck1)
		return assert.NoError(t, err)
	})

	// Get lock, check counter value
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		lck1, err := GetLockedResource(sess, "test-1", 1)
		return assert.NoError(t, err) && assert.NotEmpty(t, lck1) && assert.Equal(t, int64(1), lck1.Counter)
	})

	// Attempt temp lock on an existing key, expect error
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		err := TempLockResource(sess, "test-1", 1)
		// Must give error
		return assert.Error(t, err)
	})

	// Delete lock
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		lck1, err := GetLockedResource(sess, "test-1", 1)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, lck1) {
			return false
		}
		return assert.NoError(t, DeleteLockedResource(sess, lck1))
	})

	// Attempt temp lock on an valid key, expect success
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		return assert.NoError(t, TempLockResource(sess, "test-1", 1))
	})

	// Note: testing the validity of the locking mechanism (i.e. whether it actually locks)
	// is be done at the integration tests to ensure that all the supported databases are checked.
}
