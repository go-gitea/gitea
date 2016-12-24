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
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/db"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/sqlexec"
	"github.com/pingcap/tidb/util/types"
)

// SimpleExec represents simple statement executor.
// For statements do simple execution.
// includes `UseStmt`, 'SetStmt`, `SetCharsetStmt`.
// `DoStmt`, `BeginStmt`, `CommitStmt`, `RollbackStmt`.
// TODO: list all simple statements.
type SimpleExec struct {
	Statement ast.StmtNode
	ctx       context.Context
	done      bool
}

// Fields implements Executor Fields interface.
func (e *SimpleExec) Fields() []*ast.ResultField {
	return nil
}

// Next implements Execution Next interface.
func (e *SimpleExec) Next() (*Row, error) {
	if e.done {
		return nil, nil
	}
	var err error
	switch x := e.Statement.(type) {
	case *ast.UseStmt:
		err = e.executeUse(x)
	case *ast.SetStmt:
		err = e.executeSet(x)
	case *ast.SetCharsetStmt:
		err = e.executeSetCharset(x)
	case *ast.DoStmt:
		err = e.executeDo(x)
	case *ast.BeginStmt:
		err = e.executeBegin(x)
	case *ast.CommitStmt:
		err = e.executeCommit(x)
	case *ast.RollbackStmt:
		err = e.executeRollback(x)
	case *ast.CreateUserStmt:
		err = e.executeCreateUser(x)
	case *ast.SetPwdStmt:
		err = e.executeSetPwd(x)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	e.done = true
	return nil, nil
}

// Close implements Executor Close interface.
func (e *SimpleExec) Close() error {
	return nil
}

func (e *SimpleExec) executeUse(s *ast.UseStmt) error {
	dbname := model.NewCIStr(s.DBName)
	dbinfo, exists := sessionctx.GetDomain(e.ctx).InfoSchema().SchemaByName(dbname)
	if !exists {
		return infoschema.DatabaseNotExists.Gen("database %s not exists", dbname)
	}
	db.BindCurrentSchema(e.ctx, dbname.O)
	// character_set_database is the character set used by the default database.
	// The server sets this variable whenever the default database changes.
	// See: http://dev.mysql.com/doc/refman/5.7/en/server-system-variables.html#sysvar_character_set_database
	sessionVars := variable.GetSessionVars(e.ctx)
	sessionVars.Systems[variable.CharsetDatabase] = dbinfo.Charset
	sessionVars.Systems[variable.CollationDatabase] = dbinfo.Collate
	return nil
}

func (e *SimpleExec) executeSet(s *ast.SetStmt) error {
	sessionVars := variable.GetSessionVars(e.ctx)
	globalVars := variable.GetGlobalVarAccessor(e.ctx)
	for _, v := range s.Variables {
		// Variable is case insensitive, we use lower case.
		name := strings.ToLower(v.Name)
		if !v.IsSystem {
			// User variable.
			value, err := evaluator.Eval(e.ctx, v.Value)
			if err != nil {
				return errors.Trace(err)
			}

			if value == nil {
				delete(sessionVars.Users, name)
			} else {
				sessionVars.Users[name] = fmt.Sprintf("%v", value)
			}
			return nil
		}
		sysVar := variable.GetSysVar(name)
		if sysVar == nil {
			return variable.UnknownSystemVar.Gen("Unknown system variable '%s'", name)
		}
		if sysVar.Scope == variable.ScopeNone {
			return errors.Errorf("Variable '%s' is a read only variable", name)
		}
		if v.IsGlobal {
			if sysVar.Scope&variable.ScopeGlobal > 0 {
				value, err := evaluator.Eval(e.ctx, v.Value)
				if err != nil {
					return errors.Trace(err)
				}
				if value == nil {
					value = ""
				}
				svalue, err := types.ToString(value)
				if err != nil {
					return errors.Trace(err)
				}
				err = globalVars.SetGlobalSysVar(e.ctx, name, svalue)
				return errors.Trace(err)
			}
			return errors.Errorf("Variable '%s' is a SESSION variable and can't be used with SET GLOBAL", name)
		}
		if sysVar.Scope&variable.ScopeSession > 0 {
			if value, err := evaluator.Eval(e.ctx, v.Value); err != nil {
				return errors.Trace(err)
			} else if value == nil {
				sessionVars.Systems[name] = ""
			} else {
				sessionVars.Systems[name] = fmt.Sprintf("%v", value)
			}
			return nil
		}
		return errors.Errorf("Variable '%s' is a GLOBAL variable and should be set with SET GLOBAL", name)
	}
	return nil
}

func (e *SimpleExec) executeSetCharset(s *ast.SetCharsetStmt) error {
	collation := s.Collate
	if len(collation) == 0 {
		var err error
		collation, err = charset.GetDefaultCollation(s.Charset)
		if err != nil {
			return errors.Trace(err)
		}
	}
	sessionVars := variable.GetSessionVars(e.ctx)
	for _, v := range variable.SetNamesVariables {
		sessionVars.Systems[v] = s.Charset
	}
	sessionVars.Systems[variable.CollationConnection] = collation
	return nil
}

func (e *SimpleExec) executeDo(s *ast.DoStmt) error {
	for _, expr := range s.Exprs {
		_, err := evaluator.Eval(e.ctx, expr)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (e *SimpleExec) executeBegin(s *ast.BeginStmt) error {
	_, err := e.ctx.GetTxn(true)
	if err != nil {
		return errors.Trace(err)
	}
	// With START TRANSACTION, autocommit remains disabled until you end
	// the transaction with COMMIT or ROLLBACK. The autocommit mode then
	// reverts to its previous state.
	variable.GetSessionVars(e.ctx).SetStatusFlag(mysql.ServerStatusInTrans, true)
	return nil
}

func (e *SimpleExec) executeCommit(s *ast.CommitStmt) error {
	err := e.ctx.FinishTxn(false)
	variable.GetSessionVars(e.ctx).SetStatusFlag(mysql.ServerStatusInTrans, false)
	return errors.Trace(err)
}

func (e *SimpleExec) executeRollback(s *ast.RollbackStmt) error {
	err := e.ctx.FinishTxn(true)
	variable.GetSessionVars(e.ctx).SetStatusFlag(mysql.ServerStatusInTrans, false)
	return errors.Trace(err)
}

func (e *SimpleExec) executeCreateUser(s *ast.CreateUserStmt) error {
	users := make([]string, 0, len(s.Specs))
	for _, spec := range s.Specs {
		userName, host := parseUser(spec.User)
		exists, err1 := userExists(e.ctx, userName, host)
		if err1 != nil {
			return errors.Trace(err1)
		}
		if exists {
			if !s.IfNotExists {
				return errors.New("Duplicate user")
			}
			continue
		}
		pwd := ""
		if spec.AuthOpt.ByAuthString {
			pwd = util.EncodePassword(spec.AuthOpt.AuthString)
		} else {
			pwd = util.EncodePassword(spec.AuthOpt.HashString)
		}
		user := fmt.Sprintf(`("%s", "%s", "%s")`, host, userName, pwd)
		users = append(users, user)
	}
	if len(users) == 0 {
		return nil
	}
	sql := fmt.Sprintf(`INSERT INTO %s.%s (Host, User, Password) VALUES %s;`, mysql.SystemDB, mysql.UserTable, strings.Join(users, ", "))
	_, err := e.ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(e.ctx, sql)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// parse user string into username and host
// root@localhost -> roor, localhost
func parseUser(user string) (string, string) {
	strs := strings.Split(user, "@")
	return strs[0], strs[1]
}

func userExists(ctx context.Context, name string, host string) (bool, error) {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND Host="%s";`, mysql.SystemDB, mysql.UserTable, name, host)
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

func (e *SimpleExec) executeSetPwd(s *ast.SetPwdStmt) error {
	// TODO: If len(s.User) == 0, use CURRENT_USER()
	userName, host := parseUser(s.User)
	// Update mysql.user
	sql := fmt.Sprintf(`UPDATE %s.%s SET password="%s" WHERE User="%s" AND Host="%s";`, mysql.SystemDB, mysql.UserTable, util.EncodePassword(s.Password), userName, host)
	_, err := e.ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(e.ctx, sql)
	return errors.Trace(err)
}
