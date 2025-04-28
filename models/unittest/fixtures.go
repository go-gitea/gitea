// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type FixturesLoader interface {
	Load() error
}

var fixturesLoader FixturesLoader

// GetXORMEngine gets the XORM engine
func GetXORMEngine() (x *xorm.Engine) {
	return db.GetEngine(db.DefaultContext).(*xorm.Engine)
}

func loadFixtureResetSeqPgsql(e *xorm.Engine) error {
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
		return fmt.Errorf("failed to generate sequence update: %w", err)
	}
	for _, r := range results {
		for _, value := range r {
			_, err = e.Exec(value)
			if err != nil {
				return fmt.Errorf("failed to update sequence: %s, error: %w", value, err)
			}
		}
	}
	return nil
}

// InitFixtures initialize test fixtures for a test database
func InitFixtures(opts FixturesOptions, engine ...*xorm.Engine) (err error) {
	xormEngine := util.IfZero(util.OptionalArg(engine), GetXORMEngine())
	fixturesLoader, err = NewFixturesLoader(xormEngine, opts)
	// fixturesLoader = NewFixturesLoaderVendor(xormEngine, opts)

	// register the dummy hash algorithm function used in the test fixtures
	_ = hash.Register("dummy", hash.NewDummyHasher)
	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")
	return err
}

// LoadFixtures load fixtures for a test database
func LoadFixtures() error {
	if err := fixturesLoader.Load(); err != nil {
		return err
	}
	// Now if we're running postgres we need to tell it to update the sequences
	if GetXORMEngine().Dialect().URI().DBType == schemas.POSTGRES {
		if err := loadFixtureResetSeqPgsql(GetXORMEngine()); err != nil {
			return err
		}
	}
	return nil
}
