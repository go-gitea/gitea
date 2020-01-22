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
		resource, err := GetLockedResource(sess, "test-1", 1)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, resource) || !assert.Equal(t, int64(0), resource.Counter) {
			return false
		}
		resource.Counter++
		err = resource.UpdateValue()
		return assert.NoError(t, err)
	})

	// Get lock, check counter value
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		resource, err := GetLockedResource(sess, "test-1", 1)
		return assert.NoError(t, err) && assert.NotEmpty(t, resource) && assert.Equal(t, int64(1), resource.Counter)
	})

	// Make sure LockKey == 0 is supported and we're not
	// affecting other records
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		resource, err := GetLockedResource(sess, "test-1", 0)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, resource) || !assert.Equal(t, int64(0), resource.Counter) {
			return false
		}
		resource.Counter = 77
		return assert.NoError(t, resource.UpdateValue())
	})
	resource, err := GetLockedResource(x, "test-1", 0)
	assert.NoError(t, err)
	assert.NotEmpty(t, resource)
	assert.Equal(t, int64(77), resource.Counter)

	assert.NoError(t, DeleteLockedResourceKey(x, "test-1", 0))
	AssertExistsAndLoadBean(t, &LockedResource{LockType: "test-1", LockKey: 1})

	// Attempt temp lock on an existing key, expect error
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		err := TemporarilyLockResourceKey(sess, "test-1", 1)
		// Must give error
		return assert.Error(t, err)
	})

	// Delete lock
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		resource, err := GetLockedResource(sess, "test-1", 1)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, resource) {
			return false
		}
		return assert.NoError(t, resource.Delete())
	})
	AssertNotExistsBean(t, &LockedResource{LockType: "test-1", LockKey: 1})

	// Get lock, then delete by key
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		resource, err := GetLockedResource(sess, "test-1", 2)
		return assert.NoError(t, err) && assert.NotEmpty(t, resource)
	})
	AssertExistsAndLoadBean(t, &LockedResource{LockType: "test-1", LockKey: 2})
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		return assert.NoError(t, DeleteLockedResourceKey(sess, "test-1", 2))
	})
	AssertNotExistsBean(t, &LockedResource{LockType: "test-1", LockKey: 2})

	// Attempt temp lock on an valid key, expect success
	withSession(t, func(t *testing.T, sess *xorm.Session) bool {
		return assert.NoError(t, TemporarilyLockResourceKey(sess, "test-1", 1))
	})

	// Note: testing the validity of the locking mechanism (i.e. whether it actually locks)
	// is be done at the integration tests to ensure that all the supported databases are checked.
}
