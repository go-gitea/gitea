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

package plan

import (
	"math"

	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/util/types"
)

// Refine tries to build index or table range.
func Refine(p Plan) error {
	r := refiner{}
	p.Accept(&r)
	return r.err
}

type refiner struct {
	err error
}

func (r *refiner) Enter(in Plan) (Plan, bool) {
	return in, false
}

func (r *refiner) Leave(in Plan) (Plan, bool) {
	switch x := in.(type) {
	case *IndexScan:
		r.buildIndexRange(x)
	case *Limit:
		x.SetLimit(0)
	case *TableScan:
		r.buildTableRange(x)
	}
	return in, r.err == nil
}

var fullRange = []rangePoint{
	{start: true},
	{value: types.MaxValueDatum()},
}

func (r *refiner) buildIndexRange(p *IndexScan) {
	rb := rangeBuilder{}
	if p.AccessEqualCount > 0 {
		// Build ranges for equal access conditions.
		point := rb.build(p.AccessConditions[0])
		p.Ranges = rb.buildIndexRanges(point)
		for i := 1; i < p.AccessEqualCount; i++ {
			point = rb.build(p.AccessConditions[i])
			p.Ranges = rb.appendIndexRanges(p.Ranges, point)
		}
	}
	rangePoints := fullRange
	// Build rangePoints for non-equal access condtions.
	for i := p.AccessEqualCount; i < len(p.AccessConditions); i++ {
		rangePoints = rb.intersection(rangePoints, rb.build(p.AccessConditions[i]))
	}
	if p.AccessEqualCount == 0 {
		p.Ranges = rb.buildIndexRanges(rangePoints)
	} else if p.AccessEqualCount < len(p.AccessConditions) {
		p.Ranges = rb.appendIndexRanges(p.Ranges, rangePoints)
	}
	r.err = rb.err
	return
}

func (r *refiner) buildTableRange(p *TableScan) {
	if len(p.AccessConditions) == 0 {
		p.Ranges = []TableRange{{math.MinInt64, math.MaxInt64}}
		return
	}
	rb := rangeBuilder{}
	rangePoints := fullRange
	for _, cond := range p.AccessConditions {
		rangePoints = rb.intersection(rangePoints, rb.build(cond))
	}
	p.Ranges = rb.buildTableRanges(rangePoints)
	r.err = rb.err
}

// conditionChecker checks if this condition can be pushed to index plan.
type conditionChecker struct {
	tableName model.CIStr
	idx       *model.IndexInfo
	// the offset of the indexed column to be checked.
	columnOffset int
	pkName       model.CIStr
}

func (c *conditionChecker) check(condition ast.ExprNode) bool {
	switch x := condition.(type) {
	case *ast.BinaryOperationExpr:
		return c.checkBinaryOperation(x)
	case *ast.BetweenExpr:
		if ast.IsPreEvaluable(x.Left) && ast.IsPreEvaluable(x.Right) && c.checkColumnExpr(x.Expr) {
			return true
		}
	case *ast.ColumnNameExpr:
		return c.checkColumnExpr(x)
	case *ast.IsNullExpr:
		if c.checkColumnExpr(x.Expr) {
			return true
		}
	case *ast.IsTruthExpr:
		if c.checkColumnExpr(x.Expr) {
			return true
		}
	case *ast.ParenthesesExpr:
		return c.check(x.Expr)
	case *ast.PatternInExpr:
		if x.Sel != nil || x.Not {
			return false
		}
		if !c.checkColumnExpr(x.Expr) {
			return false
		}
		for _, val := range x.List {
			if !ast.IsPreEvaluable(val) {
				return false
			}
		}
		return true
	case *ast.PatternLikeExpr:
		if x.Not {
			return false
		}
		if !c.checkColumnExpr(x.Expr) {
			return false
		}
		if !ast.IsPreEvaluable(x.Pattern) {
			return false
		}
		patternVal := x.Pattern.GetValue()
		if patternVal == nil {
			return false
		}
		patternStr, err := types.ToString(patternVal)
		if err != nil {
			return false
		}
		firstChar := patternStr[0]
		return firstChar != '%' && firstChar != '.'
	}
	return false
}

func (c *conditionChecker) checkBinaryOperation(b *ast.BinaryOperationExpr) bool {
	switch b.Op {
	case opcode.OrOr:
		return c.check(b.L) && c.check(b.R)
	case opcode.AndAnd:
		return c.check(b.L) && c.check(b.R)
	case opcode.EQ, opcode.NE, opcode.GE, opcode.GT, opcode.LE, opcode.LT:
		if ast.IsPreEvaluable(b.L) {
			return c.checkColumnExpr(b.R)
		} else if ast.IsPreEvaluable(b.R) {
			return c.checkColumnExpr(b.L)
		}
	}
	return false
}

func (c *conditionChecker) checkColumnExpr(expr ast.ExprNode) bool {
	cn, ok := expr.(*ast.ColumnNameExpr)
	if !ok {
		return false
	}
	if cn.Refer.Table.Name.L != c.tableName.L {
		return false
	}
	if c.pkName.L != "" {
		return c.pkName.L == cn.Refer.Column.Name.L
	}
	if c.idx != nil {
		return cn.Refer.Column.Name.L == c.idx.Columns[c.columnOffset].Name.L
	}
	return true
}
