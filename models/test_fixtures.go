// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"gopkg.in/testfixtures.v2"
)

var fixtures *testfixtures.Context

// InitFixtures initialize test fixtures for a test database
func InitFixtures(helper testfixtures.Helper, dir string) (err error) {
	testfixtures.SkipDatabaseNameCheck(true)
	fixtures, err = testfixtures.NewFolder(x.DB().DB, helper, dir)
	return err
}

// LoadFixtures load fixtures for a test database
func LoadFixtures() error {
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
	if x.Dialect().DriverName() == "postgres" {
		results, err := x.QueryString(`SELECT 'SELECT SETVAL(' ||
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
				_, err = x.Exec(value)
				if err != nil {
					fmt.Printf("Failed to update sequence: %s Error: %v\n", value, err)
					return err
				}
			}
		}
	}
	return err
}
