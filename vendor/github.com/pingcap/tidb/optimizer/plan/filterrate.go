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
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/parser/opcode"
)

const (
	rateFull          float64 = 1
	rateEqual         float64 = 0.01
	rateNotEqual      float64 = 0.99
	rateBetween       float64 = 0.1
	rateGreaterOrLess float64 = 0.33
	rateIsFalse       float64 = 0.1
	rateIsNull        float64 = 0.1
	rateLike          float64 = 0.1
)

// guesstimateFilterRate guesstimates the filter rate for an expression.
// For example: a table has 100 rows, after filter expression 'a between 0 and 9',
// 10 rows returned, then the filter rate is '0.1'.
// It only depends on the expression type, not the expression value.
// The expr parameter should contain only one column name.
func guesstimateFilterRate(expr ast.ExprNode) float64 {
	switch x := expr.(type) {
	case *ast.BetweenExpr:
		return rateBetween
	case *ast.BinaryOperationExpr:
		return guesstimateBinop(x)
	case *ast.ColumnNameExpr:
		return rateFull
	case *ast.IsNullExpr:
		return guesstimateIsNull(x)
	case *ast.IsTruthExpr:
		return guesstimateIsTrue(x)
	case *ast.ParenthesesExpr:
		return guesstimateFilterRate(x.Expr)
	case *ast.PatternInExpr:
		return guesstimatePatternIn(x)
	case *ast.PatternLikeExpr:
		return guesstimatePatternLike(x)
	}
	return rateFull
}

func guesstimateBinop(expr *ast.BinaryOperationExpr) float64 {
	switch expr.Op {
	case opcode.AndAnd:
		// P(A and B) = P(A) * P(B)
		return guesstimateFilterRate(expr.L) * guesstimateFilterRate(expr.R)
	case opcode.OrOr:
		// P(A or B) = P(A) + P(B) – P(A and B)
		rateL := guesstimateFilterRate(expr.L)
		rateR := guesstimateFilterRate(expr.R)
		return rateL + rateR - rateL*rateR
	case opcode.EQ:
		return rateEqual
	case opcode.GT, opcode.GE, opcode.LT, opcode.LE:
		return rateGreaterOrLess
	case opcode.NE:
		return rateNotEqual
	}
	return rateFull
}

func guesstimateIsNull(expr *ast.IsNullExpr) float64 {
	if expr.Not {
		return rateFull - rateIsNull
	}
	return rateIsNull
}

func guesstimateIsTrue(expr *ast.IsTruthExpr) float64 {
	if expr.True == 0 {
		if expr.Not {
			return rateFull - rateIsFalse
		}
		return rateIsFalse
	}
	if expr.Not {
		return rateIsFalse + rateIsNull
	}
	return rateFull - rateIsFalse - rateIsNull
}

func guesstimatePatternIn(expr *ast.PatternInExpr) float64 {
	if len(expr.List) > 0 {
		rate := rateEqual * float64(len(expr.List))
		if expr.Not {
			return rateFull - rate
		}
		return rate
	}
	return rateFull
}

func guesstimatePatternLike(expr *ast.PatternLikeExpr) float64 {
	if expr.Not {
		return rateFull - rateLike
	}
	return rateLike
}
