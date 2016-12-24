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
	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/optimizer/plan"
	"github.com/pingcap/tidb/util/types"
)

var _ ast.SubqueryExec = &subquery{}

// subquery is an exprNode with a plan.
type subquery struct {
	types.Datum
	Type *types.FieldType
	flag uint64
	text string
	plan plan.Plan
	is   infoschema.InfoSchema
}

// SetDatum implements Expression interface.
func (sq *subquery) SetDatum(datum types.Datum) {
	sq.Datum = datum
}

// GetDatum implements Expression interface.
func (sq *subquery) GetDatum() *types.Datum {
	return &sq.Datum
}

// SetFlag implements Expression interface.
func (sq *subquery) SetFlag(flag uint64) {
	sq.flag = flag
}

// GetFlag implements Expression interface.
func (sq *subquery) GetFlag() uint64 {
	return sq.flag
}

// SetText implements Node interface.
func (sq *subquery) SetText(text string) {
	sq.text = text
}

// Text implements Node interface.
func (sq *subquery) Text() string {
	return sq.text
}

// SetType implements Expression interface.
func (sq *subquery) SetType(tp *types.FieldType) {
	sq.Type = tp
}

// GetType implements Expression interface.
func (sq *subquery) GetType() *types.FieldType {
	return sq.Type
}

func (sq *subquery) Accept(v ast.Visitor) (ast.Node, bool) {
	// SubQuery is not a normal ExprNode.
	newNode, skipChildren := v.Enter(sq)
	if skipChildren {
		return v.Leave(newNode)
	}
	sq = newNode.(*subquery)
	return v.Leave(sq)
}

func (sq *subquery) EvalRows(ctx context.Context, rowCount int) ([]interface{}, error) {
	b := newExecutorBuilder(ctx, sq.is)
	plan.Refine(sq.plan)
	e := b.build(sq.plan)
	if b.err != nil {
		return nil, errors.Trace(b.err)
	}
	defer e.Close()
	if len(e.Fields()) == 0 {
		// No result fields means no Recordset.
		for {
			row, err := e.Next()
			if err != nil {
				return nil, errors.Trace(err)
			}
			if row == nil {
				return nil, nil
			}
		}
	}
	var (
		err  error
		row  *Row
		rows = []interface{}{}
	)
	for rowCount != 0 {
		row, err = e.Next()
		if err != nil {
			return rows, errors.Trace(err)
		}
		if row == nil {
			break
		}
		if len(row.Data) == 1 {
			rows = append(rows, row.Data[0].GetValue())
		} else {
			rows = append(rows, types.DatumsToInterfaces(row.Data))
		}
		if rowCount > 0 {
			rowCount--
		}
	}
	return rows, nil
}

func (sq *subquery) ColumnCount() (int, error) {
	return len(sq.plan.Fields()), nil
}

type subqueryBuilder struct {
	is infoschema.InfoSchema
}

func (sb *subqueryBuilder) Build(p plan.Plan) ast.SubqueryExec {
	return &subquery{
		is:   sb.is,
		plan: p,
	}
}
