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

package optimizer

import (
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
)

// logicOptimize does logic optimization works on AST.
func logicOptimize(ctx context.Context, node ast.Node) error {
	return preEvaluate(ctx, node)
}

// preEvaluate evaluates preEvaluable expression and rewrites constant expression to value expression.
func preEvaluate(ctx context.Context, node ast.Node) error {
	pe := preEvaluator{ctx: ctx}
	node.Accept(&pe)
	return pe.err
}

type preEvaluator struct {
	ctx context.Context
	err error
}

func (r *preEvaluator) Enter(in ast.Node) (ast.Node, bool) {
	return in, false
}

func (r *preEvaluator) Leave(in ast.Node) (ast.Node, bool) {
	if expr, ok := in.(ast.ExprNode); ok {
		if _, ok = expr.(*ast.ValueExpr); ok {
			return in, true
		} else if ast.IsPreEvaluable(expr) {
			val, err := evaluator.Eval(r.ctx, expr)
			if err != nil {
				r.err = err
				return in, false
			}
			if ast.IsConstant(expr) {
				// The expression is constant, rewrite the expression to value expression.
				valExpr := &ast.ValueExpr{}
				valExpr.SetText(expr.Text())
				valExpr.SetType(expr.GetType())
				valExpr.SetValue(val)
				return valExpr, true
			}
			expr.SetValue(val)
		}
	}
	return in, true
}
