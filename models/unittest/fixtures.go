// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//nolint:forbidigo
package unittest

import (
	"fmt"
	"os"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-testfixtures/testfixtures/v3"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

var fixturesLoader *testfixtures.Loader

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
	var fixtureOptionFiles func(*testfixtures.Loader) error
	if opts.Dir != "" {
		fixtureOptionFiles = testfixtures.Directory(opts.Dir)
	} else {
		fixtureOptionFiles = testfixtures.Files(opts.Files...)
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
		fixtureOptionFiles,
	}

	if e.Dialect().URI().DBType == schemas.POSTGRES {
		loaderOptions = append(loaderOptions, testfixtures.SkipResetSequences())
	}

	fixturesLoader, err = testfixtures.New(loaderOptions...)
	if err != nil {
		return err
	}

	// register the dummy hash algorithm function used in the test fixtures
	_ = hash.Register("dummy", hash.NewDummyHasher)

	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")

	return err
}

// LoadFixtures load fixtures for a test database
func LoadFixtures(engine ...*xorm.Engine) error {
	e := GetXORMEngine(engine...)
	var err error
	// (doubt) database transaction conflicts could occur and result in ROLLBACK? just try for a few times.
	for i := 0; i < 5; i++ {
		if err = fixturesLoader.Load(); err == nil {
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
	_ = hash.Register("dummy", hash.NewDummyHasher)
	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")

	return err
}
