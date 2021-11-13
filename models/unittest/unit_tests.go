// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittest

import (
	"math"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/unittestbridge"
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
func BeanExists(t unittestbridge.Tester, bean interface{}, conditions ...interface{}) bool {
	ta := unittestbridge.NewAsserter(t)
	exists, err := LoadBeanIfExists(bean, conditions...)
	ta.NoError(err)
	return exists
}

// AssertExistsAndLoadBean assert that a bean exists and load it from the test database
func AssertExistsAndLoadBean(t unittestbridge.Tester, bean interface{}, conditions ...interface{}) interface{} {
	ta := unittestbridge.NewAsserter(t)
	exists, err := LoadBeanIfExists(bean, conditions...)
	ta.NoError(err)
	ta.True(exists,
		"Expected to find %+v (of type %T, with conditions %+v), but did not",
		bean, bean, conditions)
	return bean
}

// GetCount get the count of a bean
func GetCount(t unittestbridge.Tester, bean interface{}, conditions ...interface{}) int {
	ta := unittestbridge.NewAsserter(t)
	e := db.GetEngine(db.DefaultContext)
	count, err := whereConditions(e, conditions).Count(bean)
	ta.NoError(err)
	return int(count)
}

// AssertNotExistsBean assert that a bean does not exist in the test database
func AssertNotExistsBean(t unittestbridge.Tester, bean interface{}, conditions ...interface{}) {
	ta := unittestbridge.NewAsserter(t)
	exists, err := LoadBeanIfExists(bean, conditions...)
	ta.NoError(err)
	ta.False(exists)
}

// AssertExistsIf asserts that a bean exists or does not exist, depending on
// what is expected.
func AssertExistsIf(t unittestbridge.Tester, expected bool, bean interface{}, conditions ...interface{}) {
	ta := unittestbridge.NewAsserter(t)
	exists, err := LoadBeanIfExists(bean, conditions...)
	ta.NoError(err)
	ta.Equal(expected, exists)
}

// AssertSuccessfulInsert assert that beans is successfully inserted
func AssertSuccessfulInsert(t unittestbridge.Tester, beans ...interface{}) {
	ta := unittestbridge.NewAsserter(t)
	err := db.Insert(db.DefaultContext, beans...)
	ta.NoError(err)
}

// AssertCount assert the count of a bean
func AssertCount(t unittestbridge.Tester, bean, expected interface{}) {
	ta := unittestbridge.NewAsserter(t)
	ta.EqualValues(expected, GetCount(ta, bean))
}

// AssertInt64InRange assert value is in range [low, high]
func AssertInt64InRange(t unittestbridge.Tester, low, high, value int64) {
	ta := unittestbridge.NewAsserter(t)
	ta.True(value >= low && value <= high,
		"Expected value in range [%d, %d], found %d", low, high, value)
}

// GetCountByCond get the count of database entries matching bean
func GetCountByCond(ta unittestbridge.Asserter, e db.Engine, tableName string, cond builder.Cond) int64 {
	count, err := e.Table(tableName).Where(cond).Count()
	ta.NoError(err)
	return count
}

// AssertCount test the count of database entries matching bean
func AssertCountByCond(ta unittestbridge.Asserter, tableName string, cond builder.Cond, expected int) {
	ta.EqualValues(expected, GetCount(ta, db.GetEngine(db.DefaultContext), tableName, cond),
		"Failed consistency test, the counted bean (of table %s) was %+v", tableName, cond)
}
