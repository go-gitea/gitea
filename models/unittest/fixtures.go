// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
	"xorm.io/xorm/contexts"
	"xorm.io/xorm/schemas"
)

type FixturesLoader interface {
	Load() error
	MarkTableChanged(tableName string)
}

var fixturesLoader FixturesLoader

// GetXORMEngine gets the XORM engine
func GetXORMEngine() (x *xorm.Engine) {
	return db.GetXORMEngineForTesting()
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

type fixturesHookStruct struct{}

func cutSpaceForSQL(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	pos := strings.IndexFunc(s, unicode.IsSpace)
	if pos == -1 {
		return s, "", false
	}
	return s[:pos], strings.TrimSpace(s[pos+1:]), true
}

func trimTableNameQuotes(s string) string {
	pos := strings.IndexByte(s, '.')
	if pos != -1 {
		s = s[pos+1:]
	}
	return strings.Trim(s, "\"`[]")
}

func (f fixturesHookStruct) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	if c.Ctx.Value(db.ContextKeyTestFixtures) != nil {
		return c.Ctx, nil
	}
	ctx, sql := c.Ctx, c.SQL
	cmdPart, cmdRemaining, ok := cutSpaceForSQL(sql)
	if !ok {
		return ctx, nil
	}

	// ignore the SQLs which don't change data
	if util.AsciiEqualFold(cmdPart, "SELECT") ||
		util.AsciiEqualFold(cmdPart, "SHOW") ||
		util.AsciiEqualFold(cmdPart, "PRAGMA") ||
		util.AsciiEqualFold(cmdPart, "ALTER") ||
		util.AsciiEqualFold(cmdPart, "CREATE") ||
		util.AsciiEqualFold(cmdPart, "DROP") ||
		util.AsciiEqualFold(cmdPart, "IF") ||
		util.AsciiEqualFold(cmdPart, "SET") ||
		util.AsciiEqualFold(cmdPart, "sp_rename") ||
		util.AsciiEqualFold(cmdPart, "BEGIN") ||
		util.AsciiEqualFold(cmdPart, "ROLLBACK") ||
		util.AsciiEqualFold(cmdPart, "COMMIT") {
		return ctx, nil
	}

	switch {
	case util.AsciiEqualFold(cmdPart, "INSERT"):
		cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		if util.AsciiEqualFold(cmdPart, "INTO") {
			cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		}
		fixturesLoader.MarkTableChanged(trimTableNameQuotes(cmdPart))
	case util.AsciiEqualFold(cmdPart, "MERGE"):
		cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		if util.AsciiEqualFold(cmdPart, "INTO") {
			cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		}
		fixturesLoader.MarkTableChanged(trimTableNameQuotes(cmdPart))
	case util.AsciiEqualFold(cmdPart, "UPDATE"):
		cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		fixturesLoader.MarkTableChanged(trimTableNameQuotes(cmdPart))
	case util.AsciiEqualFold(cmdPart, "DELETE"):
		cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		if util.AsciiEqualFold(cmdPart, "FROM") {
			cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		}
		fixturesLoader.MarkTableChanged(trimTableNameQuotes(cmdPart))
	case util.AsciiEqualFold(cmdPart, "TRUNCATE"):
		cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		if util.AsciiEqualFold(cmdPart, "TABLE") {
			cmdPart, cmdRemaining, _ = cutSpaceForSQL(cmdRemaining)
		}
		fixturesLoader.MarkTableChanged(trimTableNameQuotes(cmdPart))
	default:
		// should either parse the table name if it changes data, or ignore it
		panic("unrecognized sql: " + sql)
	}
	_ = cmdRemaining
	return ctx, nil
}

func (f fixturesHookStruct) AfterProcess(c *contexts.ContextHook) error {
	return nil
}

// InitFixtures initialize test fixtures for a test database
func InitFixtures(opts FixturesOptions) (err error) {
	xormEngine := GetXORMEngine()
	fixturesLoader, err = NewFixturesLoader(xormEngine, opts)
	// fixturesLoader = NewFixturesLoaderVendor(xormEngine, opts)

	// register the dummy hash algorithm function used in the test fixtures
	_ = hash.Register("dummy", hash.NewDummyHasher)
	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")
	xormEngine.AddHook(&fixturesHookStruct{})
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
