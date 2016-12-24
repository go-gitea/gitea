// Copyright 2015 PingCAP, Inc.
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

package sqlexec

import (
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
)

// RestrictedSQLExecutor is an interface provides executing restricted sql statement.
// Why we need this interface?
// When we execute some management statements, we need to operate system tables.
// For example when executing create user statement, we need to check if the user already
// exists in the mysql.User table and insert a new row if not exists. In this case, we need
// a convenience way to manipulate system tables. The most simple way is executing sql statement.
// In order to execute sql statement in stmts package, we add this interface to solve dependence problem.
// And in the same time, we do not want this interface becomes a general way to run sql statement.
// We hope this could be used with some restrictions such as only allowing system tables as target,
// do not allowing recursion call.
// For more infomation please refer to the comments in session.ExecRestrictedSQL().
// This is implemented in session.go.
type RestrictedSQLExecutor interface {
	// ExecRestrictedSQL run sql statement in ctx with some restriction.
	ExecRestrictedSQL(ctx context.Context, sql string) (ast.RecordSet, error)
}
