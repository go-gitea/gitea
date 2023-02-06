// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"fmt"
	"os"
	"time"

	"code.gitea.io/gitea/models/db"

	"github.com/go-testfixtures/testfixtures/v3"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

var fixtures *testfixtures.Loader

// GetXORMEngine gets the XORM engine
func GetXORMEngine(engine ...*xorm.Engine) (x *xorm.Engine) {
	if len(engine) == 1 {
		return engine[0]
	}
	return db.DefaultContext.(*db.Context).Engine().(*xorm.Engine)
}

// InitFixtures initialize test fixtures for a test database
func InitFixtures(opts FixturesOptions, engine ...*xorm.Engine) (err error) {
	e := GetXORMEngine(engine...)
	var testfiles func(*testfixtures.Loader) error
	if opts.Dir != "" {
		testfiles = testfixtures.Directory(opts.Dir)
	} else {
		testfiles = testfixtures.Files(opts.Files...)
	}
	dialect := "unknown"
	switch e.Dialect().URI().DBType {
	case schemas.POSTGRES:
		dialect = "postgres"
	case schemas.MYSQL:
		dialect = "mysql"
	case schemas.MSSQL:
		dialect = "mssql"
	case schemas.SQLITE:
		dialect = "sqlite3"
	default:
		fmt.Println("Unsupported RDBMS for integration tests")
		os.Exit(1)
	}
	loaderOptions := []func(loader *testfixtures.Loader) error{
		testfixtures.Database(e.DB().DB),
		testfixtures.Dialect(dialect),
		testfixtures.DangerousSkipTestDatabaseCheck(),
		testfiles,
	}

	if e.Dialect().URI().DBType == schemas.POSTGRES {
		loaderOptions = append(loaderOptions, testfixtures.SkipResetSequences())
	}

	fixtures, err = testfixtures.New(loaderOptions...)
	if err != nil {
		return err
	}

	return err
}

// LoadFixtures load fixtures for a test database
func LoadFixtures(engine ...*xorm.Engine) error {
	e := GetXORMEngine(engine...)
	var err error
	// Database transaction conflicts could occur and result in ROLLBACK
	// As a simple workaround, we just retry 20 times.
	for i := 0; i < 20; i++ {
		err = fixtures.Load()
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		fmt.Printf("LoadFixtures failed after retries: %v\n", err)
	}
	// Now if we're running postgres we need to tell it to update the sequences
	if e.Dialect().URI().DBType == schemas.POSTGRES {
		results, err := e.QueryString(`SELECT 'SELECT SETVAL(' ||
		quote_literal(quote_ident(PGT.schemaname) || '.' || quote_ident(S.relname)) ||
		', COALESCE(MAX(' ||quote_ident(C.attname)|| '), 1) ) FROM ' ||
		quote_ident(PGT.schemaname)|| '.'||quote_ident(T.relname)|| ';'
	 FROM pg_class AS S,
	      pg_depend AS D,
	      pg_class AS T,
	      pg_attribute AS C,
	      pg_tables AS PGT
	 WHERE S.relkind = 'S'
	     AND S.oid = D.objid
	     AND D.refobjid = T.oid
	     AND D.refobjid = C.attrelid
	     AND D.refobjsubid = C.attnum
	     AND T.relname = PGT.tablename
	 ORDER BY S.relname;`)
		if err != nil {
			fmt.Printf("Failed to generate sequence update: %v\n", err)
			return err
		}
		for _, r := range results {
			for _, value := range r {
				_, err = e.Exec(value)
				if err != nil {
					fmt.Printf("Failed to update sequence: %s Error: %v\n", value, err)
					return err
				}
			}
		}
	}
	return err
}
