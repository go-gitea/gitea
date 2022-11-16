// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"math"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

// Code in this file is mainly used by unittest.CheckConsistencyFor, which is not in the unit test for various reasons.
// In the future if we can decouple CheckConsistencyFor into separate unit test code, then this file can be moved into unittest package too.

// NonexistentID an ID that will never exist
const NonexistentID = int64(math.MaxInt64)

type testCond struct {
	query interface{}
	args  []interface{}
}

// Cond create a condition with arguments for a test
func Cond(query interface{}, args ...interface{}) interface{} {
	return &testCond{query: query, args: args}
}

func whereConditions(e db.Engine, conditions []interface{}) db.Engine {
	for _, condition := range conditions {
		switch cond := condition.(type) {
		case *testCond:
			e = e.Where(cond.query, cond.args...)
		default:
			e = e.Where(cond)
		}
	}
	return e
}

// LoadBeanIfExists loads beans from fixture database if exist
func LoadBeanIfExists(bean interface{}, conditions ...interface{}) (bool, error) {
	e := db.GetEngine(db.DefaultContext)
	return whereConditions(e, conditions).Get(bean)
}

// BeanExists for testing, check if a bean exists
func BeanExists(t assert.TestingT, bean interface{}, conditions ...interface{}) bool {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	return exists
}

// AssertExistsAndLoadBean assert that a bean exists and load it from the test database
func AssertExistsAndLoadBean[T any](t assert.TestingT, bean T, conditions ...interface{}) T {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.True(t, exists,
		"Expected to find %+v (of type %T, with conditions %+v), but did not",
		bean, bean, conditions)
	return bean
}

// AssertExistsAndLoadMap assert that a row exists and load it from the test database
func AssertExistsAndLoadMap(t assert.TestingT, table string, conditions ...interface{}) map[string]string {
	e := db.GetEngine(db.DefaultContext).Table(table)
	res, err := whereConditions(e, conditions).Query()
	assert.NoError(t, err)
	assert.True(t, len(res) == 1,
		"Expected to find one row in %s (with conditions %+v), but found %d",
		table, conditions, len(res),
	)

	if len(res) == 1 {
		rec := map[string]string{}
		for k, v := range res[0] {
			rec[k] = string(v)
		}
		return rec
	}
	return nil
}

// GetCount get the count of a bean
func GetCount(t assert.TestingT, bean interface{}, conditions ...interface{}) int {
	e := db.GetEngine(db.DefaultContext)
	count, err := whereConditions(e, conditions).Count(bean)
	assert.NoError(t, err)
	return int(count)
}

// AssertNotExistsBean assert that a bean does not exist in the test database
func AssertNotExistsBean(t assert.TestingT, bean interface{}, conditions ...interface{}) {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// AssertExistsIf asserts that a bean exists or does not exist, depending on
// what is expected.
func AssertExistsIf(t assert.TestingT, expected bool, bean interface{}, conditions ...interface{}) {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.Equal(t, expected, exists)
}

// AssertSuccessfulInsert assert that beans is successfully inserted
func AssertSuccessfulInsert(t assert.TestingT, beans ...interface{}) {
	err := db.Insert(db.DefaultContext, beans...)
	assert.NoError(t, err)
}

// AssertCount assert the count of a bean
func AssertCount(t assert.TestingT, bean, expected interface{}) {
	assert.EqualValues(t, expected, GetCount(t, bean))
}

// AssertInt64InRange assert value is in range [low, high]
func AssertInt64InRange(t assert.TestingT, low, high, value int64) {
	assert.True(t, value >= low && value <= high,
		"Expected value in range [%d, %d], found %d", low, high, value)
}

// GetCountByCond get the count of database entries matching bean
func GetCountByCond(t assert.TestingT, tableName string, cond builder.Cond) int64 {
	e := db.GetEngine(db.DefaultContext)
	count, err := e.Table(tableName).Where(cond).Count()
	assert.NoError(t, err)
	return count
}

// AssertCountByCond test the count of database entries matching bean
func AssertCountByCond(t assert.TestingT, tableName string, cond builder.Cond, expected int) {
	assert.EqualValues(t, expected, GetCountByCond(t, tableName, cond),
		"Failed consistency test, the counted bean (of table %s) was %+v", tableName, cond)
}
