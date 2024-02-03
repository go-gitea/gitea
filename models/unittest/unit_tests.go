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
	query any
	args  []any
}

type testOrderBy string

// Cond create a condition with arguments for a test
func Cond(query any, args ...any) any {
	return &testCond{query: query, args: args}
}

// OrderBy creates "ORDER BY" a test query
func OrderBy(orderBy string) any {
	return testOrderBy(orderBy)
}

func whereOrderConditions(e db.Engine, conditions []any) db.Engine {
	orderBy := "id" // query must have the "ORDER BY", otherwise the result is not deterministic
	for _, condition := range conditions {
		switch cond := condition.(type) {
		case *testCond:
			e = e.Where(cond.query, cond.args...)
		case testOrderBy:
			orderBy = string(cond)
		default:
			e = e.Where(cond)
		}
	}
	return e.OrderBy(orderBy)
}

// LoadBeanIfExists loads beans from fixture database if exist
func LoadBeanIfExists(bean any, conditions ...any) (bool, error) {
	e := db.GetEngine(db.DefaultContext)
	return whereOrderConditions(e, conditions).Get(bean)
}

// BeanExists for testing, check if a bean exists
func BeanExists(t assert.TestingT, bean any, conditions ...any) bool {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	return exists
}

// AssertExistsAndLoadBean assert that a bean exists and load it from the test database
func AssertExistsAndLoadBean[T any](t assert.TestingT, bean T, conditions ...any) T {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.True(t, exists,
		"Expected to find %+v (of type %T, with conditions %+v), but did not",
		bean, bean, conditions)
	return bean
}

// AssertExistsAndLoadMap assert that a row exists and load it from the test database
func AssertExistsAndLoadMap(t assert.TestingT, table string, conditions ...any) map[string]string {
	e := db.GetEngine(db.DefaultContext).Table(table)
	res, err := whereOrderConditions(e, conditions).Query()
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
func GetCount(t assert.TestingT, bean any, conditions ...any) int {
	e := db.GetEngine(db.DefaultContext)
	for _, condition := range conditions {
		switch cond := condition.(type) {
		case *testCond:
			e = e.Where(cond.query, cond.args...)
		default:
			e = e.Where(cond)
		}
	}
	count, err := e.Count(bean)
	assert.NoError(t, err)
	return int(count)
}

// AssertNotExistsBean assert that a bean does not exist in the test database
func AssertNotExistsBean(t assert.TestingT, bean any, conditions ...any) {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// AssertExistsIf asserts that a bean exists or does not exist, depending on
// what is expected.
func AssertExistsIf(t assert.TestingT, expected bool, bean any, conditions ...any) {
	exists, err := LoadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.Equal(t, expected, exists)
}

// AssertSuccessfulInsert assert that beans is successfully inserted
func AssertSuccessfulInsert(t assert.TestingT, beans ...any) {
	err := db.Insert(db.DefaultContext, beans...)
	assert.NoError(t, err)
}

// AssertCount assert the count of a bean
func AssertCount(t assert.TestingT, bean, expected any) {
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
