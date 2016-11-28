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
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/privilege"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/terror"
)

// DDLExec represents a DDL executor.
type DDLExec struct {
	Statement ast.StmtNode
	ctx       context.Context
	is        infoschema.InfoSchema
	done      bool
}

// Fields implements Executor Fields interface.
func (e *DDLExec) Fields() []*ast.ResultField {
	return nil
}

// Next implements Execution Next interface.
func (e *DDLExec) Next() (*Row, error) {
	if e.done {
		return nil, nil
	}
	var err error
	switch x := e.Statement.(type) {
	case *ast.TruncateTableStmt:
		err = e.executeTruncateTable(x)
	case *ast.CreateDatabaseStmt:
		err = e.executeCreateDatabase(x)
	case *ast.CreateTableStmt:
		err = e.executeCreateTable(x)
	case *ast.CreateIndexStmt:
		err = e.executeCreateIndex(x)
	case *ast.DropDatabaseStmt:
		err = e.executeDropDatabase(x)
	case *ast.DropTableStmt:
		err = e.executeDropTable(x)
	case *ast.DropIndexStmt:
		err = e.executeDropIndex(x)
	case *ast.AlterTableStmt:
		err = e.executeAlterTable(x)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	e.done = true
	return nil, nil
}

// Close implements Executor Close interface.
func (e *DDLExec) Close() error {
	return nil
}

func (e *DDLExec) executeTruncateTable(s *ast.TruncateTableStmt) error {
	table, ok := e.is.TableByID(s.Table.TableInfo.ID)
	if !ok {
		return errors.New("table not found, should never happen")
	}
	return table.Truncate(e.ctx)
}

func (e *DDLExec) executeCreateDatabase(s *ast.CreateDatabaseStmt) error {
	var opt *ast.CharsetOpt
	if len(s.Options) != 0 {
		opt = &ast.CharsetOpt{}
		for _, val := range s.Options {
			switch val.Tp {
			case ast.DatabaseOptionCharset:
				opt.Chs = val.Value
			case ast.DatabaseOptionCollate:
				opt.Col = val.Value
			}
		}
	}
	err := sessionctx.GetDomain(e.ctx).DDL().CreateSchema(e.ctx, model.NewCIStr(s.Name), opt)
	if err != nil {
		if terror.ErrorEqual(err, infoschema.DatabaseExists) && s.IfNotExists {
			err = nil
		}
	}
	return errors.Trace(err)
}

func (e *DDLExec) executeCreateTable(s *ast.CreateTableStmt) error {
	ident := ast.Ident{Schema: s.Table.Schema, Name: s.Table.Name}
	err := sessionctx.GetDomain(e.ctx).DDL().CreateTable(e.ctx, ident, s.Cols, s.Constraints, s.Options)
	if terror.ErrorEqual(err, infoschema.TableExists) {
		if s.IfNotExists {
			return nil
		}
		return infoschema.TableExists.Gen("CREATE TABLE: table exists %s", ident)
	}
	return errors.Trace(err)
}

func (e *DDLExec) executeCreateIndex(s *ast.CreateIndexStmt) error {
	ident := ast.Ident{Schema: s.Table.Schema, Name: s.Table.Name}
	err := sessionctx.GetDomain(e.ctx).DDL().CreateIndex(e.ctx, ident, s.Unique, model.NewCIStr(s.IndexName), s.IndexColNames)
	return errors.Trace(err)
}

func (e *DDLExec) executeDropDatabase(s *ast.DropDatabaseStmt) error {
	err := sessionctx.GetDomain(e.ctx).DDL().DropSchema(e.ctx, model.NewCIStr(s.Name))
	if terror.ErrorEqual(err, infoschema.DatabaseNotExists) {
		if s.IfExists {
			err = nil
		} else {
			err = infoschema.DatabaseDropExists.Gen("Can't drop database '%s'; database doesn't exist", s.Name)
		}
	}
	return errors.Trace(err)
}

func (e *DDLExec) executeDropTable(s *ast.DropTableStmt) error {
	var notExistTables []string
	for _, tn := range s.Tables {
		fullti := ast.Ident{Schema: tn.Schema, Name: tn.Name}
		schema, ok := e.is.SchemaByName(tn.Schema)
		if !ok {
			// TODO: we should return special error for table not exist, checking "not exist" is not enough,
			// because some other errors may contain this error string too.
			notExistTables = append(notExistTables, fullti.String())
			continue
		}
		tb, err := e.is.TableByName(tn.Schema, tn.Name)
		if err != nil && strings.HasSuffix(err.Error(), "not exist") {
			notExistTables = append(notExistTables, fullti.String())
			continue
		} else if err != nil {
			return errors.Trace(err)
		}
		// Check Privilege
		privChecker := privilege.GetPrivilegeChecker(e.ctx)
		hasPriv, err := privChecker.Check(e.ctx, schema, tb.Meta(), mysql.DropPriv)
		if err != nil {
			return errors.Trace(err)
		}
		if !hasPriv {
			return errors.Errorf("You do not have the privilege to drop table %s.%s.", tn.Schema, tn.Name)
		}

		err = sessionctx.GetDomain(e.ctx).DDL().DropTable(e.ctx, fullti)
		if infoschema.DatabaseNotExists.Equal(err) || infoschema.TableNotExists.Equal(err) {
			notExistTables = append(notExistTables, fullti.String())
		} else if err != nil {
			return errors.Trace(err)
		}
	}
	if len(notExistTables) > 0 && !s.IfExists {
		return infoschema.TableDropExists.Gen("DROP TABLE: table %s does not exist", strings.Join(notExistTables, ","))
	}
	return nil
}

func (e *DDLExec) executeDropIndex(s *ast.DropIndexStmt) error {
	ti := ast.Ident{Schema: s.Table.Schema, Name: s.Table.Name}
	err := sessionctx.GetDomain(e.ctx).DDL().DropIndex(e.ctx, ti, model.NewCIStr(s.IndexName))
	if (infoschema.DatabaseNotExists.Equal(err) || infoschema.TableNotExists.Equal(err)) && s.IfExists {
		err = nil
	}
	return errors.Trace(err)
}

func (e *DDLExec) executeAlterTable(s *ast.AlterTableStmt) error {
	ti := ast.Ident{Schema: s.Table.Schema, Name: s.Table.Name}
	err := sessionctx.GetDomain(e.ctx).DDL().AlterTable(e.ctx, ti, s.Specs)
	return errors.Trace(err)
}

func joinColumnName(columnName *ast.ColumnName) string {
	var originStrs []string
	if columnName.Schema.O != "" {
		originStrs = append(originStrs, columnName.Schema.O)
	}
	if columnName.Table.O != "" {
		originStrs = append(originStrs, columnName.Table.O)
	}
	originStrs = append(originStrs, columnName.Name.O)
	return strings.Join(originStrs, ".")
}
