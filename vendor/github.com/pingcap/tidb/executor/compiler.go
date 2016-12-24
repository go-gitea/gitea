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

package executor

import (
	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/optimizer"
	"github.com/pingcap/tidb/optimizer/plan"
	"github.com/pingcap/tidb/sessionctx"
)

// Compiler compiles an ast.StmtNode to a stmt.Statement.
type Compiler struct {
}

// Compile compiles an ast.StmtNode to a stmt.Statement.
// If it is supported to use new plan and executer, it optimizes the node to
// a plan, and we wrap the plan in an adapter as stmt.Statement.
// If it is not supported, the node will be converted to old statement.
func (c *Compiler) Compile(ctx context.Context, node ast.StmtNode) (ast.Statement, error) {
	ast.SetFlag(node)

	is := sessionctx.GetDomain(ctx).InfoSchema()
	if err := optimizer.Preprocess(node, is, ctx); err != nil {
		return nil, errors.Trace(err)
	}
	// Validate should be after NameResolve.
	if err := optimizer.Validate(node, false); err != nil {
		return nil, errors.Trace(err)
	}
	sb := NewSubQueryBuilder(is)
	p, err := optimizer.Optimize(ctx, node, sb)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sa := &statement{
		is:   is,
		plan: p,
	}
	return sa, nil
}

// NewSubQueryBuilder builds and returns a new SubQuery builder.
func NewSubQueryBuilder(is infoschema.InfoSchema) plan.SubQueryBuilder {
	return &subqueryBuilder{is: is}
}
