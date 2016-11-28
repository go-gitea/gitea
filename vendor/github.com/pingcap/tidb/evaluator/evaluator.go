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

package evaluator

import (
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util/types"
)

// Error instances.
var (
	ErrInvalidOperation = terror.ClassEvaluator.New(CodeInvalidOperation, "invalid operation")
)

// Error codes.
const (
	CodeInvalidOperation terror.ErrCode = 1
)

// Eval evaluates an expression to a value.
func Eval(ctx context.Context, expr ast.ExprNode) (interface{}, error) {
	e := &Evaluator{ctx: ctx}
	expr.Accept(e)
	if e.err != nil {
		return nil, errors.Trace(e.err)
	}
	return expr.GetValue(), nil
}

// EvalBool evalueates an expression to a boolean value.
func EvalBool(ctx context.Context, expr ast.ExprNode) (bool, error) {
	val, err := Eval(ctx, expr)
	if err != nil {
		return false, errors.Trace(err)
	}
	if val == nil {
		return false, nil
	}

	i, err := types.ToBool(val)
	if err != nil {
		return false, errors.Trace(err)
	}
	return i != 0, nil
}

func boolToInt64(v bool) int64 {
	if v {
		return int64(1)
	}
	return int64(0)
}

// Evaluator is an ast Visitor that evaluates an expression.
type Evaluator struct {
	ctx          context.Context
	err          error
	multipleRows bool
	existRow     bool
}

// Enter implements ast.Visitor interface.
func (e *Evaluator) Enter(in ast.Node) (out ast.Node, skipChildren bool) {
	switch v := in.(type) {
	case *ast.SubqueryExpr:
		if v.Evaluated && !v.UseOuterContext {
			return in, true
		}
	case *ast.PatternInExpr, *ast.CompareSubqueryExpr:
		e.multipleRows = true
	case *ast.ExistsSubqueryExpr:
		e.existRow = true
	}
	return in, false
}

// Leave implements ast.Visitor interface.
func (e *Evaluator) Leave(in ast.Node) (out ast.Node, ok bool) {
	switch v := in.(type) {
	case *ast.AggregateFuncExpr:
		ok = e.aggregateFunc(v)
	case *ast.BetweenExpr:
		ok = e.between(v)
	case *ast.BinaryOperationExpr:
		ok = e.binaryOperation(v)
	case *ast.CaseExpr:
		ok = e.caseExpr(v)
	case *ast.ColumnName:
		ok = true
	case *ast.ColumnNameExpr:
		ok = e.columnName(v)
	case *ast.CompareSubqueryExpr:
		e.multipleRows = false
		ok = e.compareSubquery(v)
	case *ast.DefaultExpr:
		ok = e.defaultExpr(v)
	case *ast.ExistsSubqueryExpr:
		e.existRow = false
		ok = e.existsSubquery(v)
	case *ast.FuncCallExpr:
		ok = e.funcCall(v)
	case *ast.FuncCastExpr:
		ok = e.funcCast(v)
	case *ast.IsNullExpr:
		ok = e.isNull(v)
	case *ast.IsTruthExpr:
		ok = e.isTruth(v)
	case *ast.ParamMarkerExpr:
		ok = e.paramMarker(v)
	case *ast.ParenthesesExpr:
		ok = e.parentheses(v)
	case *ast.PatternInExpr:
		e.multipleRows = false
		ok = e.patternIn(v)
	case *ast.PatternLikeExpr:
		ok = e.patternLike(v)
	case *ast.PatternRegexpExpr:
		ok = e.patternRegexp(v)
	case *ast.PositionExpr:
		ok = e.position(v)
	case *ast.RowExpr:
		ok = e.row(v)
	case *ast.SubqueryExpr:
		ok = e.subqueryExpr(v)
	case ast.SubqueryExec:
		ok = e.subqueryExec(v)
	case *ast.UnaryOperationExpr:
		ok = e.unaryOperation(v)
	case *ast.ValueExpr:
		ok = true
	case *ast.ValuesExpr:
		ok = e.values(v)
	case *ast.VariableExpr:
		ok = e.variable(v)
	case *ast.WhenClause:
		ok = true
	}
	out = in
	return
}

func (e *Evaluator) between(v *ast.BetweenExpr) bool {
	var l, r ast.ExprNode
	op := opcode.AndAnd

	if v.Not {
		// v < lv || v > rv
		op = opcode.OrOr
		l = &ast.BinaryOperationExpr{Op: opcode.LT, L: v.Expr, R: v.Left}
		r = &ast.BinaryOperationExpr{Op: opcode.GT, L: v.Expr, R: v.Right}
	} else {
		// v >= lv && v <= rv
		l = &ast.BinaryOperationExpr{Op: opcode.GE, L: v.Expr, R: v.Left}
		r = &ast.BinaryOperationExpr{Op: opcode.LE, L: v.Expr, R: v.Right}
	}

	ret := &ast.BinaryOperationExpr{Op: op, L: l, R: r}
	ret.Accept(e)
	if e.err != nil {
		return false
	}
	v.SetDatum(*ret.GetDatum())
	return true
}

func (e *Evaluator) caseExpr(v *ast.CaseExpr) bool {
	tmp := types.NewDatum(boolToInt64(true))
	target := &tmp
	if v.Value != nil {
		target = v.Value.GetDatum()
	}
	if target.Kind() != types.KindNull {
		for _, val := range v.WhenClauses {
			cmp, err := target.CompareDatum(*val.Expr.GetDatum())
			if err != nil {
				e.err = errors.Trace(err)
				return false
			}
			if cmp == 0 {
				v.SetDatum(*val.Result.GetDatum())
				return true
			}
		}
	}
	if v.ElseClause != nil {
		v.SetDatum(*v.ElseClause.GetDatum())
	} else {
		v.SetNull()
	}
	return true
}

func (e *Evaluator) columnName(v *ast.ColumnNameExpr) bool {
	v.SetDatum(*v.Refer.Expr.GetDatum())
	return true
}

func (e *Evaluator) defaultExpr(v *ast.DefaultExpr) bool {
	return true
}

func (e *Evaluator) compareSubquery(cs *ast.CompareSubqueryExpr) bool {
	lvDatum := cs.L.GetDatum()
	if lvDatum.Kind() == types.KindNull {
		cs.SetNull()
		return true
	}
	lv := lvDatum.GetValue()
	x, err := e.checkResult(cs, lv, cs.R.GetValue().([]interface{}))
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}
	cs.SetValue(x)
	return true
}

func (e *Evaluator) checkResult(cs *ast.CompareSubqueryExpr, lv interface{}, result []interface{}) (interface{}, error) {
	if cs.All {
		return e.checkAllResult(cs, lv, result)
	}
	return e.checkAnyResult(cs, lv, result)
}

func (e *Evaluator) checkAllResult(cs *ast.CompareSubqueryExpr, lv interface{}, result []interface{}) (interface{}, error) {
	hasNull := false
	for _, v := range result {
		if v == nil {
			hasNull = true
			continue
		}

		comRes, err := types.Compare(lv, v)
		if err != nil {
			return nil, errors.Trace(err)
		}

		res, err := getCompResult(cs.Op, comRes)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !res {
			return false, nil
		}
	}
	if hasNull {
		// If no matched but we get null, return null.
		// Like `insert t (c) values (1),(2),(null)`, then
		// `select 3 > all (select c from t)`, returns null.
		return nil, nil
	}
	return true, nil
}

func (e *Evaluator) checkAnyResult(cs *ast.CompareSubqueryExpr, lv interface{}, result []interface{}) (interface{}, error) {
	hasNull := false
	for _, v := range result {
		if v == nil {
			hasNull = true
			continue
		}

		comRes, err := types.Compare(lv, v)
		if err != nil {
			return nil, errors.Trace(err)
		}

		res, err := getCompResult(cs.Op, comRes)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if res {
			return true, nil
		}
	}

	if hasNull {
		// If no matched but we get null, return null.
		// Like `insert t (c) values (1),(2),(null)`, then
		// `select 0 > any (select c from t)`, returns null.
		return nil, nil
	}

	return false, nil
}

func (e *Evaluator) existsSubquery(v *ast.ExistsSubqueryExpr) bool {
	datum := v.Sel.GetDatum()
	if datum.Kind() == types.KindNull {
		v.SetInt64(0)
		return true
	}
	r := datum.GetValue()
	rows, _ := r.([]interface{})
	if len(rows) > 0 {
		v.SetInt64(1)
	} else {
		v.SetInt64(0)
	}
	return true
}

// Evaluate SubqueryExpr.
// Get the value from v.SubQuery and set it to v.
func (e *Evaluator) subqueryExpr(v *ast.SubqueryExpr) bool {
	if v.SubqueryExec != nil {
		v.SetDatum(*v.SubqueryExec.GetDatum())
	}
	v.Evaluated = true
	return true
}

// Do the real work to evaluate subquery.
func (e *Evaluator) subqueryExec(v ast.SubqueryExec) bool {
	rowCount := 2
	if e.multipleRows {
		rowCount = -1
	} else if e.existRow {
		rowCount = 1
	}
	rows, err := v.EvalRows(e.ctx, rowCount)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}
	if e.multipleRows || e.existRow {
		v.SetValue(rows)
		return true
	}
	switch len(rows) {
	case 0:
		v.GetDatum().SetNull()
	case 1:
		v.SetValue(rows[0])
	default:
		e.err = errors.New("Subquery returns more than 1 row")
		return false
	}
	return true
}

func (e *Evaluator) checkInList(not bool, in interface{}, list []interface{}) interface{} {
	hasNull := false
	for _, v := range list {
		if v == nil {
			hasNull = true
			continue
		}

		r, err := types.Compare(types.Coerce(in, v))
		if err != nil {
			e.err = errors.Trace(err)
			return nil
		}
		if r == 0 {
			if !not {
				return 1
			}
			return 0
		}
	}

	if hasNull {
		// if no matched but we got null in In, return null
		// e.g 1 in (null, 2, 3) returns null
		return nil
	}
	if not {
		return 1
	}
	return 0
}

func (e *Evaluator) patternIn(n *ast.PatternInExpr) bool {
	lhs := n.Expr.GetDatum()
	if lhs.Kind() == types.KindNull {
		n.SetNull()
		return true
	}
	if n.Sel == nil {
		values := make([]interface{}, 0, len(n.List))
		for _, ei := range n.List {
			values = append(values, ei.GetValue())
		}
		x := e.checkInList(n.Not, lhs.GetValue(), values)
		if e.err != nil {
			return false
		}
		n.SetValue(x)
		return true
	}
	se := n.Sel.(*ast.SubqueryExpr)
	sel := se.SubqueryExec

	res := sel.GetValue().([]interface{})
	x := e.checkInList(n.Not, lhs.GetValue(), res)
	if e.err != nil {
		return false
	}
	n.SetValue(x)
	return true
}

func (e *Evaluator) isNull(v *ast.IsNullExpr) bool {
	var boolVal bool
	if v.Expr.GetDatum().Kind() == types.KindNull {
		boolVal = true
	}
	if v.Not {
		boolVal = !boolVal
	}
	v.SetInt64(boolToInt64(boolVal))
	return true
}

func (e *Evaluator) isTruth(v *ast.IsTruthExpr) bool {
	var boolVal bool
	datum := v.Expr.GetDatum()
	if datum.Kind() != types.KindNull {
		ival, err := datum.ToBool()
		if err != nil {
			e.err = errors.Trace(err)
			return false
		}
		if ival == v.True {
			boolVal = true
		}
	}
	if v.Not {
		boolVal = !boolVal
	}
	v.GetDatum().SetInt64(boolToInt64(boolVal))
	return true
}

func (e *Evaluator) paramMarker(v *ast.ParamMarkerExpr) bool {
	return true
}

func (e *Evaluator) parentheses(v *ast.ParenthesesExpr) bool {
	v.SetDatum(*v.Expr.GetDatum())
	return true
}

func (e *Evaluator) position(v *ast.PositionExpr) bool {
	v.SetDatum(*v.Refer.Expr.GetDatum())
	return true
}

func (e *Evaluator) row(v *ast.RowExpr) bool {
	row := make([]interface{}, 0, len(v.Values))
	for _, val := range v.Values {
		row = append(row, val.GetValue())
	}
	v.SetValue(row)
	return true
}

func (e *Evaluator) unaryOperation(u *ast.UnaryOperationExpr) bool {
	defer func() {
		if er := recover(); er != nil {
			e.err = errors.Errorf("%v", er)
		}
	}()
	aDatum := u.V.GetDatum()
	if aDatum.Kind() == types.KindNull {
		u.SetNull()
		return true
	}
	switch op := u.Op; op {
	case opcode.Not:
		n, err := aDatum.ToBool()
		if err != nil {
			e.err = errors.Trace(err)
		} else if n == 0 {
			u.SetInt64(1)
		} else {
			u.SetInt64(0)
		}
	case opcode.BitNeg:
		// for bit operation, we will use int64 first, then return uint64
		n, err := aDatum.ToInt64()
		if err != nil {
			e.err = errors.Trace(err)
			return false
		}
		u.SetUint64(uint64(^n))
	case opcode.Plus:
		switch aDatum.Kind() {
		case types.KindInt64,
			types.KindUint64,
			types.KindFloat64,
			types.KindFloat32,
			types.KindMysqlDuration,
			types.KindMysqlTime,
			types.KindString,
			types.KindMysqlDecimal,
			types.KindBytes,
			types.KindMysqlHex,
			types.KindMysqlBit,
			types.KindMysqlEnum,
			types.KindMysqlSet:
			u.SetDatum(*aDatum)
		default:
			e.err = ErrInvalidOperation
			return false
		}
	case opcode.Minus:
		switch aDatum.Kind() {
		case types.KindInt64:
			u.SetInt64(-aDatum.GetInt64())
		case types.KindUint64:
			u.SetInt64(-int64(aDatum.GetUint64()))
		case types.KindFloat64:
			u.SetFloat64(-aDatum.GetFloat64())
		case types.KindFloat32:
			u.SetFloat32(-aDatum.GetFloat32())
		case types.KindMysqlDuration:
			u.SetValue(mysql.ZeroDecimal.Sub(aDatum.GetMysqlDuration().ToNumber()))
		case types.KindMysqlTime:
			u.SetValue(mysql.ZeroDecimal.Sub(aDatum.GetMysqlTime().ToNumber()))
		case types.KindString:
			f, err := types.StrToFloat(aDatum.GetString())
			e.err = errors.Trace(err)
			u.SetFloat64(-f)
		case types.KindMysqlDecimal:
			f, _ := aDatum.GetMysqlDecimal().Float64()
			u.SetValue(mysql.NewDecimalFromFloat(-f))
		case types.KindBytes:
			f, err := types.StrToFloat(string(aDatum.GetBytes()))
			e.err = errors.Trace(err)
			u.SetFloat64(-f)
		case types.KindMysqlHex:
			u.SetFloat64(-aDatum.GetMysqlHex().ToNumber())
		case types.KindMysqlBit:
			u.SetFloat64(-aDatum.GetMysqlBit().ToNumber())
		case types.KindMysqlEnum:
			u.SetFloat64(-aDatum.GetMysqlEnum().ToNumber())
		case types.KindMysqlSet:
			u.SetFloat64(-aDatum.GetMysqlSet().ToNumber())
		default:
			e.err = ErrInvalidOperation
			return false
		}
	default:
		e.err = ErrInvalidOperation
		return false
	}

	return true
}

func (e *Evaluator) values(v *ast.ValuesExpr) bool {
	v.SetDatum(*v.Column.GetDatum())
	return true
}

func (e *Evaluator) variable(v *ast.VariableExpr) bool {
	name := strings.ToLower(v.Name)
	sessionVars := variable.GetSessionVars(e.ctx)
	globalVars := variable.GetGlobalVarAccessor(e.ctx)
	if !v.IsSystem {
		// user vars
		if value, ok := sessionVars.Users[name]; ok {
			v.SetString(value)
			return true
		}
		// select null user vars is permitted.
		v.SetNull()
		return true
	}

	_, ok := variable.SysVars[name]
	if !ok {
		// select null sys vars is not permitted
		e.err = variable.UnknownSystemVar.Gen("Unknown system variable '%s'", name)
		return false
	}

	if !v.IsGlobal {
		if value, ok := sessionVars.Systems[name]; ok {
			v.SetString(value)
			return true
		}
	}

	value, err := globalVars.GetGlobalSysVar(e.ctx, name)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	v.SetString(value)
	return true
}

func (e *Evaluator) funcCall(v *ast.FuncCallExpr) bool {
	of, ok := OldFuncs[v.FnName.L]
	if ok {
		if len(v.Args) < of.MinArgs || (of.MaxArgs != -1 && len(v.Args) > of.MaxArgs) {
			e.err = ErrInvalidOperation.Gen("number of function arguments must in [%d, %d].", of.MinArgs, of.MaxArgs)
			return false
		}
		a := make([]interface{}, len(v.Args))
		for i, arg := range v.Args {
			a[i] = arg.GetValue()
		}
		val, err := of.F(a, e.ctx)
		if err != nil {
			e.err = errors.Trace(err)
			return false
		}
		v.SetValue(val)
		return true
	}
	f, ok := Funcs[v.FnName.L]
	if !ok {
		e.err = ErrInvalidOperation.Gen("unknown function %s", v.FnName.O)
		return false
	}
	if len(v.Args) < f.MinArgs || (f.MaxArgs != -1 && len(v.Args) > f.MaxArgs) {
		e.err = ErrInvalidOperation.Gen("number of function arguments must in [%d, %d].", f.MinArgs, f.MaxArgs)
		return false
	}
	a := make([]types.Datum, len(v.Args))
	for i, arg := range v.Args {
		a[i] = *arg.GetDatum()
	}
	val, err := f.F(a, e.ctx)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}
	v.SetDatum(val)
	return true
}

func (e *Evaluator) funcCast(v *ast.FuncCastExpr) bool {
	value := v.Expr.GetValue()
	// Casting nil to any type returns null
	if value == nil {
		v.SetNull()
		return true
	}
	var err error
	value, err = types.Cast(value, v.Tp)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}
	v.SetValue(value)
	return true
}

func (e *Evaluator) aggregateFunc(v *ast.AggregateFuncExpr) bool {
	name := strings.ToLower(v.F)
	switch name {
	case ast.AggFuncAvg:
		e.evalAggAvg(v)
	case ast.AggFuncCount:
		e.evalAggCount(v)
	case ast.AggFuncFirstRow, ast.AggFuncMax, ast.AggFuncMin, ast.AggFuncSum:
		e.evalAggSetValue(v)
	case ast.AggFuncGroupConcat:
		e.evalAggGroupConcat(v)
	}
	return e.err == nil
}

func (e *Evaluator) evalAggCount(v *ast.AggregateFuncExpr) {
	ctx := v.GetContext()
	v.SetInt64(ctx.Count)
}

func (e *Evaluator) evalAggSetValue(v *ast.AggregateFuncExpr) {
	ctx := v.GetContext()
	v.SetValue(ctx.Value)
}

func (e *Evaluator) evalAggAvg(v *ast.AggregateFuncExpr) {
	ctx := v.GetContext()
	switch x := ctx.Value.(type) {
	case float64:
		ctx.Value = x / float64(ctx.Count)
	case mysql.Decimal:
		ctx.Value = x.Div(mysql.NewDecimalFromUint(uint64(ctx.Count), 0))
	}
	v.SetValue(ctx.Value)
}

func (e *Evaluator) evalAggGroupConcat(v *ast.AggregateFuncExpr) {
	ctx := v.GetContext()
	if ctx.Buffer != nil {
		v.SetValue(ctx.Buffer.String())
	} else {
		v.SetValue(nil)
	}
}
