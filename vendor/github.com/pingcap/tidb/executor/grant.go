// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package executor

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/db"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/util/sqlexec"
	"github.com/pingcap/tidb/util/types"
)

/***
 * Grant Statement
 * See: https://dev.mysql.com/doc/refman/5.7/en/grant.html
 ************************************************************************************/
var (
	_ Executor = (*GrantExec)(nil)
)

// GrantExec executes GrantStmt.
type GrantExec struct {
	Privs      []*ast.PrivElem
	ObjectType ast.ObjectTypeType
	Level      *ast.GrantLevel
	Users      []*ast.UserSpec

	ctx  context.Context
	done bool
}

// Fields implements Executor Fields interface.
func (e *GrantExec) Fields() []*ast.ResultField {
	return nil
}

// Next implements Execution Next interface.
func (e *GrantExec) Next() (*Row, error) {
	if e.done {
		return nil, nil
	}
	// Grant for each user
	for _, user := range e.Users {
		// Check if user exists.
		userName, host := parseUser(user.User)
		exists, err := userExists(e.ctx, userName, host)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !exists {
			return nil, errors.Errorf("Unknown user: %s", user.User)
		}

		// If there is no privilege entry in corresponding table, insert a new one.
		// DB scope:		mysql.DB
		// Table scope:		mysql.Tables_priv
		// Column scope:	mysql.Columns_priv
		switch e.Level.Level {
		case ast.GrantLevelDB:
			err := e.checkAndInitDBPriv(userName, host)
			if err != nil {
				return nil, errors.Trace(err)
			}
		case ast.GrantLevelTable:
			err := e.checkAndInitTablePriv(userName, host)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		// Grant each priv to the user.
		for _, priv := range e.Privs {
			if len(priv.Cols) > 0 {
				// Check column scope privilege entry.
				// TODO: Check validity before insert new entry.
				err1 := e.checkAndInitColumnPriv(userName, host, priv.Cols)
				if err1 != nil {
					return nil, errors.Trace(err1)
				}
			}
			err2 := e.grantPriv(priv, user)
			if err2 != nil {
				return nil, errors.Trace(err2)
			}
		}
	}
	e.done = true
	return nil, nil
}

// Close implements Executor Close interface.
func (e *GrantExec) Close() error {
	return nil
}

// Check if DB scope privilege entry exists in mysql.DB.
// If unexists, insert a new one.
func (e *GrantExec) checkAndInitDBPriv(user string, host string) error {
	db, err := e.getTargetSchema()
	if err != nil {
		return errors.Trace(err)
	}
	ok, err := dbUserExists(e.ctx, user, host, db.Name.O)
	if err != nil {
		return errors.Trace(err)
	}
	if ok {
		return nil
	}
	// Entry does not exist for user-host-db. Insert a new entry.
	return initDBPrivEntry(e.ctx, user, host, db.Name.O)
}

// Check if table scope privilege entry exists in mysql.Tables_priv.
// If unexists, insert a new one.
func (e *GrantExec) checkAndInitTablePriv(user string, host string) error {
	db, tbl, err := e.getTargetSchemaAndTable()
	if err != nil {
		return errors.Trace(err)
	}
	ok, err := tableUserExists(e.ctx, user, host, db.Name.O, tbl.Meta().Name.O)
	if err != nil {
		return errors.Trace(err)
	}
	if ok {
		return nil
	}
	// Entry does not exist for user-host-db-tbl. Insert a new entry.
	return initTablePrivEntry(e.ctx, user, host, db.Name.O, tbl.Meta().Name.O)
}

// Check if column scope privilege entry exists in mysql.Columns_priv.
// If unexists, insert a new one.
func (e *GrantExec) checkAndInitColumnPriv(user string, host string, cols []*ast.ColumnName) error {
	db, tbl, err := e.getTargetSchemaAndTable()
	if err != nil {
		return errors.Trace(err)
	}
	for _, c := range cols {
		col := column.FindCol(tbl.Cols(), c.Name.L)
		if col == nil {
			return errors.Errorf("Unknown column: %s", c.Name.O)
		}
		ok, err := columnPrivEntryExists(e.ctx, user, host, db.Name.O, tbl.Meta().Name.O, col.Name.O)
		if err != nil {
			return errors.Trace(err)
		}
		if ok {
			continue
		}
		// Entry does not exist for user-host-db-tbl-col. Insert a new entry.
		err = initColumnPrivEntry(e.ctx, user, host, db.Name.O, tbl.Meta().Name.O, col.Name.O)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Insert a new row into mysql.DB with empty privilege.
func initDBPrivEntry(ctx context.Context, user string, host string, db string) error {
	sql := fmt.Sprintf(`INSERT INTO %s.%s (Host, User, DB) VALUES ("%s", "%s", "%s")`, mysql.SystemDB, mysql.DBTable, host, user, db)
	_, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	return errors.Trace(err)
}

// Insert a new row into mysql.Tables_priv with empty privilege.
func initTablePrivEntry(ctx context.Context, user string, host string, db string, tbl string) error {
	sql := fmt.Sprintf(`INSERT INTO %s.%s (Host, User, DB, Table_name, Table_priv, Column_priv) VALUES ("%s", "%s", "%s", "%s", "", "")`, mysql.SystemDB, mysql.TablePrivTable, host, user, db, tbl)
	_, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	return errors.Trace(err)
}

// Insert a new row into mysql.Columns_priv with empty privilege.
func initColumnPrivEntry(ctx context.Context, user string, host string, db string, tbl string, col string) error {
	sql := fmt.Sprintf(`INSERT INTO %s.%s (Host, User, DB, Table_name, Column_name, Column_priv) VALUES ("%s", "%s", "%s", "%s", "%s", "")`, mysql.SystemDB, mysql.ColumnPrivTable, host, user, db, tbl, col)
	_, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	return errors.Trace(err)
}

// Grant priv to user in s.Level scope.
func (e *GrantExec) grantPriv(priv *ast.PrivElem, user *ast.UserSpec) error {
	switch e.Level.Level {
	case ast.GrantLevelGlobal:
		return e.grantGlobalPriv(priv, user)
	case ast.GrantLevelDB:
		return e.grantDBPriv(priv, user)
	case ast.GrantLevelTable:
		if len(priv.Cols) == 0 {
			return e.grantTablePriv(priv, user)
		}
		return e.grantColumnPriv(priv, user)
	default:
		return errors.Errorf("Unknown grant level: %#v", e.Level)
	}
}

// Manipulate mysql.user table.
func (e *GrantExec) grantGlobalPriv(priv *ast.PrivElem, user *ast.UserSpec) error {
	asgns, err := composeGlobalPrivUpdate(priv.Priv)
	if err != nil {
		return errors.Trace(err)
	}
	userName, host := parseUser(user.User)
	sql := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE User="%s" AND Host="%s"`, mysql.SystemDB, mysql.UserTable, asgns, userName, host)
	_, err = e.ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(e.ctx, sql)
	return errors.Trace(err)
}

// Manipulate mysql.db table.
func (e *GrantExec) grantDBPriv(priv *ast.PrivElem, user *ast.UserSpec) error {
	db, err := e.getTargetSchema()
	if err != nil {
		return errors.Trace(err)
	}
	asgns, err := composeDBPrivUpdate(priv.Priv)
	if err != nil {
		return errors.Trace(err)
	}
	userName, host := parseUser(user.User)
	sql := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE User="%s" AND Host="%s" AND DB="%s";`, mysql.SystemDB, mysql.DBTable, asgns, userName, host, db.Name.O)
	_, err = e.ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(e.ctx, sql)
	return errors.Trace(err)
}

// Manipulate mysql.tables_priv table.
func (e *GrantExec) grantTablePriv(priv *ast.PrivElem, user *ast.UserSpec) error {
	db, tbl, err := e.getTargetSchemaAndTable()
	if err != nil {
		return errors.Trace(err)
	}
	userName, host := parseUser(user.User)
	asgns, err := composeTablePrivUpdate(e.ctx, priv.Priv, userName, host, db.Name.O, tbl.Meta().Name.O)
	if err != nil {
		return errors.Trace(err)
	}
	sql := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE User="%s" AND Host="%s" AND DB="%s" AND Table_name="%s";`, mysql.SystemDB, mysql.TablePrivTable, asgns, userName, host, db.Name.O, tbl.Meta().Name.O)
	_, err = e.ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(e.ctx, sql)
	return errors.Trace(err)
}

// Manipulate mysql.tables_priv table.
func (e *GrantExec) grantColumnPriv(priv *ast.PrivElem, user *ast.UserSpec) error {
	db, tbl, err := e.getTargetSchemaAndTable()
	if err != nil {
		return errors.Trace(err)
	}
	userName, host := parseUser(user.User)
	for _, c := range priv.Cols {
		col := column.FindCol(tbl.Cols(), c.Name.L)
		if col == nil {
			return errors.Errorf("Unknown column: %s", c)
		}
		asgns, err := composeColumnPrivUpdate(e.ctx, priv.Priv, userName, host, db.Name.O, tbl.Meta().Name.O, col.Name.O)
		if err != nil {
			return errors.Trace(err)
		}
		sql := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE User="%s" AND Host="%s" AND DB="%s" AND Table_name="%s" AND Column_name="%s";`, mysql.SystemDB, mysql.ColumnPrivTable, asgns, userName, host, db.Name.O, tbl.Meta().Name.O, col.Name.O)
		_, err = e.ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(e.ctx, sql)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Compose update stmt assignment list string for global scope privilege update.
func composeGlobalPrivUpdate(priv mysql.PrivilegeType) (string, error) {
	if priv == mysql.AllPriv {
		strs := make([]string, 0, len(mysql.Priv2UserCol))
		for _, v := range mysql.Priv2UserCol {
			strs = append(strs, fmt.Sprintf(`%s="Y"`, v))
		}
		return strings.Join(strs, ", "), nil
	}
	col, ok := mysql.Priv2UserCol[priv]
	if !ok {
		return "", errors.Errorf("Unknown priv: %v", priv)
	}
	return fmt.Sprintf(`%s="Y"`, col), nil
}

// Compose update stmt assignment list for db scope privilege update.
func composeDBPrivUpdate(priv mysql.PrivilegeType) (string, error) {
	if priv == mysql.AllPriv {
		strs := make([]string, 0, len(mysql.AllDBPrivs))
		for _, p := range mysql.AllDBPrivs {
			v, ok := mysql.Priv2UserCol[p]
			if !ok {
				return "", errors.Errorf("Unknown db privilege %v", priv)
			}
			strs = append(strs, fmt.Sprintf(`%s="Y"`, v))
		}
		return strings.Join(strs, ", "), nil
	}
	col, ok := mysql.Priv2UserCol[priv]
	if !ok {
		return "", errors.Errorf("Unknown priv: %v", priv)
	}
	return fmt.Sprintf(`%s="Y"`, col), nil
}

// Compose update stmt assignment list for table scope privilege update.
func composeTablePrivUpdate(ctx context.Context, priv mysql.PrivilegeType, name string, host string, db string, tbl string) (string, error) {
	var newTablePriv, newColumnPriv string
	if priv == mysql.AllPriv {
		for _, p := range mysql.AllTablePrivs {
			v, ok := mysql.Priv2SetStr[p]
			if !ok {
				return "", errors.Errorf("Unknown table privilege %v", p)
			}
			if len(newTablePriv) == 0 {
				newTablePriv = v
			} else {
				newTablePriv = fmt.Sprintf("%s,%s", newTablePriv, v)
			}
		}
		for _, p := range mysql.AllColumnPrivs {
			v, ok := mysql.Priv2SetStr[p]
			if !ok {
				return "", errors.Errorf("Unknown column privilege %v", p)
			}
			if len(newColumnPriv) == 0 {
				newColumnPriv = v
			} else {
				newColumnPriv = fmt.Sprintf("%s,%s", newColumnPriv, v)
			}
		}
	} else {
		currTablePriv, currColumnPriv, err := getTablePriv(ctx, name, host, db, tbl)
		if err != nil {
			return "", errors.Trace(err)
		}
		p, ok := mysql.Priv2SetStr[priv]
		if !ok {
			return "", errors.Errorf("Unknown priv: %v", priv)
		}
		if len(currTablePriv) == 0 {
			newTablePriv = p
		} else {
			newTablePriv = fmt.Sprintf("%s,%s", currTablePriv, p)
		}
		for _, cp := range mysql.AllColumnPrivs {
			if priv == cp {
				if len(currColumnPriv) == 0 {
					newColumnPriv = p
				} else {
					newColumnPriv = fmt.Sprintf("%s,%s", currColumnPriv, p)
				}
				break
			}
		}
	}
	return fmt.Sprintf(`Table_priv="%s", Column_priv="%s", Grantor="%s"`, newTablePriv, newColumnPriv, variable.GetSessionVars(ctx).User), nil
}

// Compose update stmt assignment list for column scope privilege update.
func composeColumnPrivUpdate(ctx context.Context, priv mysql.PrivilegeType, name string, host string, db string, tbl string, col string) (string, error) {
	newColumnPriv := ""
	if priv == mysql.AllPriv {
		for _, p := range mysql.AllColumnPrivs {
			v, ok := mysql.Priv2SetStr[p]
			if !ok {
				return "", errors.Errorf("Unknown column privilege %v", p)
			}
			if len(newColumnPriv) == 0 {
				newColumnPriv = v
			} else {
				newColumnPriv = fmt.Sprintf("%s,%s", newColumnPriv, v)
			}
		}
	} else {
		currColumnPriv, err := getColumnPriv(ctx, name, host, db, tbl, col)
		if err != nil {
			return "", errors.Trace(err)
		}
		p, ok := mysql.Priv2SetStr[priv]
		if !ok {
			return "", errors.Errorf("Unknown priv: %v", priv)
		}
		if len(currColumnPriv) == 0 {
			newColumnPriv = p
		} else {
			newColumnPriv = fmt.Sprintf("%s,%s", currColumnPriv, p)
		}
	}
	return fmt.Sprintf(`Column_priv="%s"`, newColumnPriv), nil
}

// Helper function to check if the sql returns any row.
func recordExists(ctx context.Context, sql string) (bool, error) {
	rs, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return false, errors.Trace(err)
	}
	defer rs.Close()
	row, err := rs.Next()
	if err != nil {
		return false, errors.Trace(err)
	}
	return row != nil, nil
}

// Check if there is an entry with key user-host-db in mysql.DB.
func dbUserExists(ctx context.Context, name string, host string, db string) (bool, error) {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND Host="%s" AND DB="%s";`, mysql.SystemDB, mysql.DBTable, name, host, db)
	return recordExists(ctx, sql)
}

// Check if there is an entry with key user-host-db-tbl in mysql.Tables_priv.
func tableUserExists(ctx context.Context, name string, host string, db string, tbl string) (bool, error) {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND Host="%s" AND DB="%s" AND Table_name="%s";`, mysql.SystemDB, mysql.TablePrivTable, name, host, db, tbl)
	return recordExists(ctx, sql)
}

// Check if there is an entry with key user-host-db-tbl-col in mysql.Columns_priv.
func columnPrivEntryExists(ctx context.Context, name string, host string, db string, tbl string, col string) (bool, error) {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND Host="%s" AND DB="%s" AND Table_name="%s" AND Column_name="%s";`, mysql.SystemDB, mysql.ColumnPrivTable, name, host, db, tbl, col)
	return recordExists(ctx, sql)
}

// Get current table scope privilege set from mysql.Tables_priv.
// Return Table_priv and Column_priv.
func getTablePriv(ctx context.Context, name string, host string, db string, tbl string) (string, string, error) {
	sql := fmt.Sprintf(`SELECT Table_priv, Column_priv FROM %s.%s WHERE User="%s" AND Host="%s" AND DB="%s" AND Table_name="%s";`, mysql.SystemDB, mysql.TablePrivTable, name, host, db, tbl)
	rs, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	defer rs.Close()
	row, err := rs.Next()
	if err != nil {
		return "", "", errors.Trace(err)
	}
	var tPriv, cPriv string
	if row.Data[0].Kind() == types.KindMysqlSet {
		tablePriv := row.Data[0].GetMysqlSet()
		tPriv = tablePriv.Name
	}
	if row.Data[1].Kind() == types.KindMysqlSet {
		columnPriv := row.Data[1].GetMysqlSet()
		cPriv = columnPriv.Name
	}
	return tPriv, cPriv, nil
}

// Get current column scope privilege set from mysql.Columns_priv.
// Return Column_priv.
func getColumnPriv(ctx context.Context, name string, host string, db string, tbl string, col string) (string, error) {
	sql := fmt.Sprintf(`SELECT Column_priv FROM %s.%s WHERE User="%s" AND Host="%s" AND DB="%s" AND Table_name="%s" AND Column_name="%s";`, mysql.SystemDB, mysql.ColumnPrivTable, name, host, db, tbl, col)
	rs, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer rs.Close()
	row, err := rs.Next()
	if err != nil {
		return "", errors.Trace(err)
	}
	cPriv := ""
	if row.Data[0].Kind() == types.KindMysqlSet {
		cPriv = row.Data[0].GetMysqlSet().Name
	}
	return cPriv, nil
}

// Find the schema by dbName.
func (e *GrantExec) getTargetSchema() (*model.DBInfo, error) {
	dbName := e.Level.DBName
	if len(dbName) == 0 {
		// Grant *, use current schema
		dbName = db.GetCurrentSchema(e.ctx)
		if len(dbName) == 0 {
			return nil, errors.New("Miss DB name for grant privilege.")
		}
	}
	//check if db exists
	schema := model.NewCIStr(dbName)
	is := sessionctx.GetDomain(e.ctx).InfoSchema()
	db, ok := is.SchemaByName(schema)
	if !ok {
		return nil, errors.Errorf("Unknown schema name: %s", dbName)
	}
	return db, nil
}

// Find the schema and table by dbName and tableName.
func (e *GrantExec) getTargetSchemaAndTable() (*model.DBInfo, table.Table, error) {
	db, err := e.getTargetSchema()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	name := model.NewCIStr(e.Level.TableName)
	is := sessionctx.GetDomain(e.ctx).InfoSchema()
	tbl, err := is.TableByName(db.Name, name)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	return db, tbl, nil
}
