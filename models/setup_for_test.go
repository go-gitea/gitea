// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3" // for the test engine
	"github.com/stretchr/testify/assert"
	"gopkg.in/testfixtures.v2"
)

func TestMain(m *testing.M) {
	if err := CreateTestEngine(); err != nil {
		fmt.Printf("Error creating test engine: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

var fixtures *testfixtures.Context

// CreateTestEngine create an xorm engine for testing
func CreateTestEngine() error {
	testfixtures.SkipDatabaseNameCheck(true)
	var err error
	x, err = xorm.NewEngine("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		return err
	}
	x.SetMapper(core.GonicMapper{})
	if err = x.StoreEngine("InnoDB").Sync2(tables...); err != nil {
		return err
	}
	fixtures, err = testfixtures.NewFolder(x.DB().DB, &testfixtures.SQLite{}, "fixtures/")
	return err
}

// PrepareTestDatabase load test fixtures into test database
func PrepareTestDatabase() error {
	return fixtures.Load()
}

func loadBeanIfExists(bean interface{}, conditions ...interface{}) (bool, error) {
	sess := x.NewSession()
	defer sess.Close()

	for _, cond := range conditions {
		sess = sess.Where(cond)
	}
	return sess.Get(bean)
}

// AssertExistsAndLoadBean assert that a bean exists and load it from the test
// database
func AssertExistsAndLoadBean(t *testing.T, bean interface{}, conditions ...interface{}) {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.True(t, exists)
}

// AssertNotExistsBean assert that a bean does not exist in the test database
func AssertNotExistsBean(t *testing.T, bean interface{}, conditions ...interface{}) {
	exists, err := loadBeanIfExists(bean, conditions...)
	assert.NoError(t, err)
	assert.False(t, exists)
}
