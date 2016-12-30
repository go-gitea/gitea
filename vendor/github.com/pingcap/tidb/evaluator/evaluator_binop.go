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
	"math"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/util/types"
)

const (
	zeroI64 int64 = 0
	oneI64  int64 = 1
)

func (e *Evaluator) binaryOperation(o *ast.BinaryOperationExpr) bool {
	switch o.Op {
	case opcode.AndAnd, opcode.OrOr, opcode.LogicXor:
		return e.handleLogicOperation(o)
	case opcode.LT, opcode.LE, opcode.GE, opcode.GT, opcode.EQ, opcode.NE, opcode.NullEQ:
		return e.handleComparisonOp(o)
	case opcode.RightShift, opcode.LeftShift, opcode.And, opcode.Or, opcode.Xor:
		return e.handleBitOp(o)
	case opcode.Plus, opcode.Minus, opcode.Mod, opcode.Div, opcode.Mul, opcode.IntDiv:
		return e.handleArithmeticOp(o)
	default:
		e.err = ErrInvalidOperation
		return false
	}
}

func (e *Evaluator) handleLogicOperation(o *ast.BinaryOperationExpr) bool {
	switch o.Op {
	case opcode.AndAnd:
		return e.handleAndAnd(o)
	case opcode.OrOr:
		return e.handleOrOr(o)
	case opcode.LogicXor:
		return e.handleXor(o)
	default:
		e.err = ErrInvalidOperation.Gen("unkown operator %s", o.Op)
		return false
	}
}

func (e *Evaluator) handleAndAnd(o *ast.BinaryOperationExpr) bool {
	leftVal := o.L.GetValue()
	righVal := o.R.GetValue()
	if leftVal != nil {
		x, err := types.ToBool(leftVal)
		if err != nil {
			e.err = errors.Trace(err)
			return false
		} else if x == 0 {
			// false && any other types is false
			o.SetValue(x)
			return true
		}
	}
	if righVal != nil {
		y, err := types.ToBool(righVal)
		if err != nil {
			e.err = errors.Trace(err)
			return false
		} else if y == 0 {
			o.SetValue(y)
			return true
		}
	}
	if leftVal == nil || righVal == nil {
		o.SetValue(nil)
		return true
	}
	o.SetValue(int64(1))
	return true
}

func (e *Evaluator) handleOrOr(o *ast.BinaryOperationExpr) bool {
	leftVal := o.L.GetValue()
	righVal := o.R.GetValue()
	if leftVal != nil {
		x, err := types.ToBool(leftVal)
		if err != nil {
			e.err = errors.Trace(err)
			return false
		} else if x == 1 {
			// true || any other types is true.
			o.SetValue(x)
			return true
		}
	}
	if righVal != nil {
		y, err := types.ToBool(righVal)
		if err != nil {
			e.err = errors.Trace(err)
			return false
		} else if y == 1 {
			o.SetValue(y)
			return true
		}
	}
	if leftVal == nil || righVal == nil {
		o.SetValue(nil)
		return true
	}
	o.SetValue(int64(0))
	return true
}

func (e *Evaluator) handleXor(o *ast.BinaryOperationExpr) bool {
	leftVal := o.L.GetValue()
	righVal := o.R.GetValue()
	if leftVal == nil || righVal == nil {
		o.SetValue(nil)
		return true
	}
	x, err := types.ToBool(leftVal)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	y, err := types.ToBool(righVal)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}
	if x == y {
		o.SetValue(int64(0))
	} else {
		o.SetValue(int64(1))
	}
	return true
}

func (e *Evaluator) handleComparisonOp(o *ast.BinaryOperationExpr) bool {
	a, b := types.Coerce(o.L.GetValue(), o.R.GetValue())
	if a == nil || b == nil {
		// for <=>, if a and b are both nil, return true.
		// if a or b is nil, return false.
		if o.Op == opcode.NullEQ {
			if a == nil && b == nil {
				o.SetValue(oneI64)
			} else {
				o.SetValue(zeroI64)
			}
		} else {
			o.SetValue(nil)
		}
		return true
	}

	n, err := types.Compare(a, b)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	r, err := getCompResult(o.Op, n)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}
	if r {
		o.SetValue(oneI64)
	} else {
		o.SetValue(zeroI64)
	}
	return true
}

func getCompResult(op opcode.Op, value int) (bool, error) {
	switch op {
	case opcode.LT:
		return value < 0, nil
	case opcode.LE:
		return value <= 0, nil
	case opcode.GE:
		return value >= 0, nil
	case opcode.GT:
		return value > 0, nil
	case opcode.EQ:
		return value == 0, nil
	case opcode.NE:
		return value != 0, nil
	case opcode.NullEQ:
		return value == 0, nil
	default:
		return false, ErrInvalidOperation.Gen("invalid op %v in comparision operation", op)
	}
}

func (e *Evaluator) handleBitOp(o *ast.BinaryOperationExpr) bool {
	a, b := types.Coerce(o.L.GetValue(), o.R.GetValue())

	if a == nil || b == nil {
		o.SetValue(nil)
		return true
	}

	x, err := types.ToInt64(a)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	y, err := types.ToInt64(b)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	// use a int64 for bit operator, return uint64
	switch o.Op {
	case opcode.And:
		o.SetValue(uint64(x & y))
	case opcode.Or:
		o.SetValue(uint64(x | y))
	case opcode.Xor:
		o.SetValue(uint64(x ^ y))
	case opcode.RightShift:
		o.SetValue(uint64(x) >> uint64(y))
	case opcode.LeftShift:
		o.SetValue(uint64(x) << uint64(y))
	default:
		e.err = ErrInvalidOperation.Gen("invalid op %v in bit operation", o.Op)
		return false
	}
	return true
}

func (e *Evaluator) handleArithmeticOp(o *ast.BinaryOperationExpr) bool {
	a, err := coerceArithmetic(o.L.GetValue())
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	b, err := coerceArithmetic(o.R.GetValue())
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	a, b = types.Coerce(a, b)
	if a == nil || b == nil {
		o.SetValue(nil)
		return true
	}

	var result interface{}
	switch o.Op {
	case opcode.Plus:
		result, e.err = computePlus(a, b)
	case opcode.Minus:
		result, e.err = computeMinus(a, b)
	case opcode.Mul:
		result, e.err = computeMul(a, b)
	case opcode.Div:
		result, e.err = computeDiv(a, b)
	case opcode.Mod:
		result, e.err = computeMod(a, b)
	case opcode.IntDiv:
		result, e.err = computeIntDiv(a, b)
	default:
		e.err = ErrInvalidOperation.Gen("invalid op %v in arithmetic operation", o.Op)
		return false
	}
	o.SetValue(result)
	return e.err == nil
}

func computePlus(a, b interface{}) (interface{}, error) {
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			return types.AddInt64(x, y)
		case uint64:
			return types.AddInteger(y, x)
		}
	case uint64:
		switch y := b.(type) {
		case int64:
			return types.AddInteger(x, y)
		case uint64:
			return types.AddUint64(x, y)
		}
	case float64:
		switch y := b.(type) {
		case float64:
			return x + y, nil
		}
	case mysql.Decimal:
		switch y := b.(type) {
		case mysql.Decimal:
			return x.Add(y), nil
		}
	}
	return types.InvOp2(a, b, opcode.Plus)
}

func computeMinus(a, b interface{}) (interface{}, error) {
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			return types.SubInt64(x, y)
		case uint64:
			return types.SubIntWithUint(x, y)
		}
	case uint64:
		switch y := b.(type) {
		case int64:
			return types.SubUintWithInt(x, y)
		case uint64:
			return types.SubUint64(x, y)
		}
	case float64:
		switch y := b.(type) {
		case float64:
			return x - y, nil
		}
	case mysql.Decimal:
		switch y := b.(type) {
		case mysql.Decimal:
			return x.Sub(y), nil
		}
	}

	return types.InvOp2(a, b, opcode.Minus)
}

func computeMul(a, b interface{}) (interface{}, error) {
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			return types.MulInt64(x, y)
		case uint64:
			return types.MulInteger(y, x)
		}
	case uint64:
		switch y := b.(type) {
		case int64:
			return types.MulInteger(x, y)
		case uint64:
			return types.MulUint64(x, y)
		}
	case float64:
		switch y := b.(type) {
		case float64:
			return x * y, nil
		}
	case mysql.Decimal:
		switch y := b.(type) {
		case mysql.Decimal:
			return x.Mul(y), nil
		}
	}

	return types.InvOp2(a, b, opcode.Mul)
}

func computeDiv(a, b interface{}) (interface{}, error) {
	// MySQL support integer divison Div and division operator /
	// we use opcode.Div for division operator and will use another for integer division later.
	// for division operator, we will use float64 for calculation.
	switch x := a.(type) {
	case float64:
		y, err := types.ToFloat64(b)
		if err != nil {
			return nil, errors.Trace(err)
		}

		if y == 0 {
			return nil, nil
		}

		return x / y, nil
	default:
		// the scale of the result is the scale of the first operand plus
		// the value of the div_precision_increment system variable (which is 4 by default)
		// we will use 4 here
		xa, err := types.ToDecimal(a)
		if err != nil {
			return nil, errors.Trace(err)
		}

		xb, err := types.ToDecimal(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if f, _ := xb.Float64(); f == 0 {
			// division by zero return null
			return nil, nil
		}

		return xa.Div(xb), nil
	}
}

func computeMod(a, b interface{}) (interface{}, error) {
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			if y == 0 {
				return nil, nil
			}
			return x % y, nil
		case uint64:
			if y == 0 {
				return nil, nil
			} else if x < 0 {
				// first is int64, return int64.
				return -int64(uint64(-x) % y), nil
			}
			return int64(uint64(x) % y), nil
		}
	case uint64:
		switch y := b.(type) {
		case int64:
			if y == 0 {
				return nil, nil
			} else if y < 0 {
				// first is uint64, return uint64.
				return uint64(x % uint64(-y)), nil
			}
			return x % uint64(y), nil
		case uint64:
			if y == 0 {
				return nil, nil
			}
			return x % y, nil
		}
	case float64:
		switch y := b.(type) {
		case float64:
			if y == 0 {
				return nil, nil
			}
			return math.Mod(x, y), nil
		}
	case mysql.Decimal:
		switch y := b.(type) {
		case mysql.Decimal:
			xf, _ := x.Float64()
			yf, _ := y.Float64()
			if yf == 0 {
				return nil, nil
			}
			return math.Mod(xf, yf), nil
		}
	}

	return types.InvOp2(a, b, opcode.Mod)
}

func computeIntDiv(a, b interface{}) (interface{}, error) {
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			if y == 0 {
				return nil, nil
			}
			return types.DivInt64(x, y)
		case uint64:
			if y == 0 {
				return nil, nil
			}
			return types.DivIntWithUint(x, y)
		}
	case uint64:
		switch y := b.(type) {
		case int64:
			if y == 0 {
				return nil, nil
			}
			return types.DivUintWithInt(x, y)
		case uint64:
			if y == 0 {
				return nil, nil
			}
			return x / y, nil
		}
	}

	// if any is none integer, use decimal to calculate
	x, err := types.ToDecimal(a)
	if err != nil {
		return nil, errors.Trace(err)
	}

	y, err := types.ToDecimal(b)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if f, _ := y.Float64(); f == 0 {
		return nil, nil
	}

	return x.Div(y).IntPart(), nil
}

func coerceArithmetic(a interface{}) (interface{}, error) {
	switch x := a.(type) {
	case string:
		// MySQL will convert string to float for arithmetic operation
		f, err := types.StrToFloat(x)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return f, errors.Trace(err)
	case mysql.Time:
		// if time has no precision, return int64
		v := x.ToNumber()
		if x.Fsp == 0 {
			return v.IntPart(), nil
		}
		return v, nil
	case mysql.Duration:
		// if duration has no precision, return int64
		v := x.ToNumber()
		if x.Fsp == 0 {
			return v.IntPart(), nil
		}
		return v, nil
	case []byte:
		// []byte is the same as string, converted to float for arithmetic operator.
		f, err := types.StrToFloat(string(x))
		if err != nil {
			return nil, errors.Trace(err)
		}
		return f, errors.Trace(err)
	case mysql.Hex:
		return x.ToNumber(), nil
	case mysql.Bit:
		return x.ToNumber(), nil
	case mysql.Enum:
		return x.ToNumber(), nil
	case mysql.Set:
		return x.ToNumber(), nil
	default:
		return x, nil
	}
}
