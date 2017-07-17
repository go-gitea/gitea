// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/go-xorm/xorm"
	"github.com/stretchr/testify/assert"
)

// NonexistentID an ID that will never exist
const NonexistentID = 9223372036854775807

// PrepareTestDatabase load test fixtures into test database
func PrepareTestDatabase() error {
	return LoadFixtures()
}

type testCond struct {
	query interface{}
	args  []interface{}
}

// Cond create a condition with arguments for a test
func Cond(query interface{}, args ...interface{}) interface{} {
	return &testCond{query: query, args: args}
}

func whereConditions(sess *xorm.Session, conditions []interface{}) {
	for _, condition := range conditions {
		switch cond := condition.(type) {
		case *testCond:
			sess.Where(cond.query, cond.args...)
		default:
			sess.Where(cond)
		}
	}
}

func loadBeanIfExists(bean interface{}, conditions ...interface{}) (bool, error) {
	sess := x.NewSession()
	defer sess.Close()
	whereConditions(sess, conditions)
	return sess.Get(bean)
}

// BeanExists for testing, check if a bean exists
func BeanExists(t *testing.T, bean interface{}, conditions ...interface{}) bool {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	return exists
}

// AssertExistsAndLoadBean assert that a bean exists and load it from the test
// database
func AssertExistsAndLoadBean(t *testing.T, bean interface{}, conditions ...interface{}) interface{} {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.True(t, exists,
		"Expected to find %+v (of type %T, with conditions %+v), but did not",
		bean, bean, conditions)
	return bean
}

// GetCount get the count of a bean
func GetCount(t *testing.T, bean interface{}, conditions ...interface{}) int {
	sess := x.NewSession()
	defer sess.Close()
	whereConditions(sess, conditions)
	count, err := sess.Count(bean)
	assert.NoError(t, err)
	return int(count)
}

// AssertNotExistsBean assert that a bean does not exist in the test database
func AssertNotExistsBean(t *testing.T, bean interface{}, conditions ...interface{}) {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// AssertSuccessfulInsert assert that beans is successfully inserted
func AssertSuccessfulInsert(t *testing.T, beans ...interface{}) {
	_, err := x.Insert(beans...)
	assert.NoError(t, err)
}

// AssertCount assert the count of a bean
func AssertCount(t *testing.T, bean interface{}, expected interface{}) {
	assert.EqualValues(t, expected, GetCount(t, bean))
}
