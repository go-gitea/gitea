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

package ast

const preEvaluable = FlagHasParamMarker | FlagHasFunc | FlagHasVariable | FlagHasDefault

// IsPreEvaluable checks if the expression can be evaluated before execution.
func IsPreEvaluable(expr ExprNode) bool {
	return expr.GetFlag()|preEvaluable == preEvaluable
}

// IsConstant checks if the expression is constant.
// A constant expression is safe to be rewritten to value expression.
func IsConstant(expr ExprNode) bool {
	return expr.GetFlag() == FlagConstant
}

// HasAggFlag checks if the expr contains FlagHasAggregateFunc.
func HasAggFlag(expr ExprNode) bool {
	return expr.GetFlag()&FlagHasAggregateFunc > 0
}

// SetFlag sets flag for expression.
func SetFlag(n Node) {
	var setter flagSetter
	n.Accept(&setter)
}

type flagSetter struct {
}

func (f *flagSetter) Enter(in Node) (Node, bool) {
	return in, false
}

func (f *flagSetter) Leave(in Node) (Node, bool) {
	switch x := in.(type) {
	case *AggregateFuncExpr:
		f.aggregateFunc(x)
	case *BetweenExpr:
		x.SetFlag(x.Expr.GetFlag() | x.Left.GetFlag() | x.Right.GetFlag())
	case *BinaryOperationExpr:
		x.SetFlag(x.L.GetFlag() | x.R.GetFlag())
	case *CaseExpr:
		f.caseExpr(x)
	case *ColumnNameExpr:
		x.SetFlag(FlagHasReference)
	case *CompareSubqueryExpr:
		x.SetFlag(x.L.GetFlag() | x.R.GetFlag())
	case *DefaultExpr:
		x.SetFlag(FlagHasDefault)
	case *ExistsSubqueryExpr:
		x.SetFlag(x.Sel.GetFlag())
	case *FuncCallExpr:
		f.funcCall(x)
	case *FuncCastExpr:
		x.SetFlag(FlagHasFunc | x.Expr.GetFlag())
	case *IsNullExpr:
		x.SetFlag(x.Expr.GetFlag())
	case *IsTruthExpr:
		x.SetFlag(x.Expr.GetFlag())
	case *ParamMarkerExpr:
		x.SetFlag(FlagHasParamMarker)
	case *ParenthesesExpr:
		x.SetFlag(x.Expr.GetFlag())
	case *PatternInExpr:
		f.patternIn(x)
	case *PatternLikeExpr:
		f.patternLike(x)
	case *PatternRegexpExpr:
		f.patternRegexp(x)
	case *PositionExpr:
		x.SetFlag(FlagHasReference)
	case *RowExpr:
		f.row(x)
	case *SubqueryExpr:
		x.SetFlag(FlagHasSubquery)
	case *UnaryOperationExpr:
		x.SetFlag(x.V.GetFlag())
	case *ValueExpr:
	case *ValuesExpr:
		x.SetFlag(FlagHasReference)
	case *VariableExpr:
		x.SetFlag(FlagHasVariable)
	}

	return in, true
}

func (f *flagSetter) caseExpr(x *CaseExpr) {
	var flag uint64
	if x.Value != nil {
		flag |= x.Value.GetFlag()
	}
	for _, val := range x.WhenClauses {
		flag |= val.Expr.GetFlag()
		flag |= val.Result.GetFlag()
	}
	if x.ElseClause != nil {
		flag |= x.ElseClause.GetFlag()
	}
	x.SetFlag(flag)
}

func (f *flagSetter) patternIn(x *PatternInExpr) {
	flag := x.Expr.GetFlag()
	for _, val := range x.List {
		flag |= val.GetFlag()
	}
	if x.Sel != nil {
		flag |= x.Sel.GetFlag()
	}
	x.SetFlag(flag)
}

func (f *flagSetter) patternLike(x *PatternLikeExpr) {
	flag := x.Pattern.GetFlag()
	if x.Expr != nil {
		flag |= x.Expr.GetFlag()
	}
	x.SetFlag(flag)
}

func (f *flagSetter) patternRegexp(x *PatternRegexpExpr) {
	flag := x.Pattern.GetFlag()
	if x.Expr != nil {
		flag |= x.Expr.GetFlag()
	}
	x.SetFlag(flag)
}

func (f *flagSetter) row(x *RowExpr) {
	var flag uint64
	for _, val := range x.Values {
		flag |= val.GetFlag()
	}
	x.SetFlag(flag)
}

func (f *flagSetter) funcCall(x *FuncCallExpr) {
	flag := FlagHasFunc
	for _, val := range x.Args {
		flag |= val.GetFlag()
	}
	x.SetFlag(flag)
}

func (f *flagSetter) aggregateFunc(x *AggregateFuncExpr) {
	flag := FlagHasAggregateFunc
	for _, val := range x.Args {
		flag |= val.GetFlag()
	}
	x.SetFlag(flag)
}
