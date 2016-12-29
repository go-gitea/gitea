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
