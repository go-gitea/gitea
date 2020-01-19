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
		cont := f(t, sess)
		if !cont {
			return
		}
		err = sess.Commit()
		assert.NoError(t, err)
	}

	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		lck1, err := GetLockedResource(sess, "test-1",1)
		if	!assert.NoError(t, err) ||
			!assert.NotEmpty(t, lck1) ||
			!assert.Equal(t, int64(0), lck1.Counter) {
			return false
		}
		lck1.Counter++
		err = UpdateLockedResource(sess, lck1)
		return assert.NoError(t, err)
	})

	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		lck1, err := GetLockedResource(sess, "test-1",1)
		return assert.NoError(t, err) &&
			assert.NotEmpty(t, lck1) &&
			assert.Equal(t, int64(1), lck1.Counter)
	})

	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		return assert.Error(t, TempLockResource(sess, "test-1",1))
	})

	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		lck1, err := GetLockedResource(sess, "test-1",1)
		return assert.NoError(t, err) &&
			assert.NotEmpty(t, lck1) &&
			assert.NoError(t, DeleteLockedResource(sess, lck1))
	})

	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		return assert.NoError(t, TempLockResource(sess, "test-1",2))
	})

	// Note: testing the validity of the locking mechanism (i.e. whether it actually locks)
	// must be done in the integration tests, so all the supported databases are checked.
}

