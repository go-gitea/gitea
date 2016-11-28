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
	"fmt"
	"math"
	"sort"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/util/types"
)

type rangePoint struct {
	value types.Datum
	excl  bool // exclude
	start bool
}

func (rp rangePoint) String() string {
	val := rp.value.GetValue()
	if rp.value.Kind() == types.KindMinNotNull {
		val = "-inf"
	} else if rp.value.Kind() == types.KindMaxValue {
		val = "+inf"
	}
	if rp.start {
		symbol := "["
		if rp.excl {
			symbol = "("
		}
		return fmt.Sprintf("%s%v", symbol, val)
	}
	symbol := "]"
	if rp.excl {
		symbol = ")"
	}
	return fmt.Sprintf("%v%s", val, symbol)
}

type rangePointSorter struct {
	points []rangePoint
	err    error
}

func (r *rangePointSorter) Len() int {
	return len(r.points)
}

func (r *rangePointSorter) Less(i, j int) bool {
	a := r.points[i]
	b := r.points[j]
	cmp, err := a.value.CompareDatum(b.value)
	if err != nil {
		r.err = err
		return true
	}
	if cmp == 0 {
		return r.equalValueLess(a, b)
	}
	return cmp < 0
}

func (r *rangePointSorter) equalValueLess(a, b rangePoint) bool {
	if a.start && b.start {
		return !a.excl && b.excl
	} else if a.start {
		return !b.excl
	} else if b.start {
		return a.excl || b.excl
	}
	return a.excl && !b.excl
}

func (r *rangePointSorter) Swap(i, j int) {
	r.points[i], r.points[j] = r.points[j], r.points[i]
}

type rangeBuilder struct {
	err error
}

func (r *rangeBuilder) build(expr ast.ExprNode) []rangePoint {
	switch x := expr.(type) {
	case *ast.BinaryOperationExpr:
		return r.buildFromBinop(x)
	case *ast.PatternInExpr:
		return r.buildFromIn(x)
	case *ast.ParenthesesExpr:
		return r.build(x.Expr)
	case *ast.BetweenExpr:
		return r.buildFromBetween(x)
	case *ast.IsNullExpr:
		return r.buildFromIsNull(x)
	case *ast.IsTruthExpr:
		return r.buildFromIsTruth(x)
	case *ast.PatternLikeExpr:
		rans := r.buildFromPatternLike(x)
		return rans
	case *ast.ColumnNameExpr:
		return r.buildFromColumnName(x)
	}
	return fullRange
}

func (r *rangeBuilder) buildFromBinop(x *ast.BinaryOperationExpr) []rangePoint {
	if x.Op == opcode.OrOr {
		return r.union(r.build(x.L), r.build(x.R))
	} else if x.Op == opcode.AndAnd {
		return r.intersection(r.build(x.L), r.build(x.R))
	}
	// This has been checked that the binary operation is comparison operation, and one of
	// the operand is column name expression.
	var value types.Datum
	var op opcode.Op
	if _, ok := x.L.(*ast.ValueExpr); ok {
		value = types.NewDatum(x.L.GetValue())
		switch x.Op {
		case opcode.GE:
			op = opcode.LE
		case opcode.GT:
			op = opcode.LT
		case opcode.LT:
			op = opcode.GT
		case opcode.LE:
			op = opcode.GE
		default:
			op = x.Op
		}
	} else {
		value = types.NewDatum(x.R.GetValue())
		op = x.Op
	}
	if value.Kind() == types.KindNull {
		return nil
	}
	switch op {
	case opcode.EQ:
		startPoint := rangePoint{value: value, start: true}
		endPoint := rangePoint{value: value}
		return []rangePoint{startPoint, endPoint}
	case opcode.NE:
		startPoint1 := rangePoint{value: types.MinNotNullDatum(), start: true}
		endPoint1 := rangePoint{value: value, excl: true}
		startPoint2 := rangePoint{value: value, start: true, excl: true}
		endPoint2 := rangePoint{value: types.MaxValueDatum()}
		return []rangePoint{startPoint1, endPoint1, startPoint2, endPoint2}
	case opcode.LT:
		startPoint := rangePoint{value: types.MinNotNullDatum(), start: true}
		endPoint := rangePoint{value: value, excl: true}
		return []rangePoint{startPoint, endPoint}
	case opcode.LE:
		startPoint := rangePoint{value: types.MinNotNullDatum(), start: true}
		endPoint := rangePoint{value: value}
		return []rangePoint{startPoint, endPoint}
	case opcode.GT:
		startPoint := rangePoint{value: value, start: true, excl: true}
		endPoint := rangePoint{value: types.MaxValueDatum()}
		return []rangePoint{startPoint, endPoint}
	case opcode.GE:
		startPoint := rangePoint{value: value, start: true}
		endPoint := rangePoint{value: types.MaxValueDatum()}
		return []rangePoint{startPoint, endPoint}
	}
	return nil
}

func (r *rangeBuilder) buildFromIn(x *ast.PatternInExpr) []rangePoint {
	if x.Not {
		r.err = ErrUnsupportedType.Gen("NOT IN is not supported")
		return fullRange
	}
	var rangePoints []rangePoint
	for _, v := range x.List {
		startPoint := rangePoint{value: types.NewDatum(v.GetValue()), start: true}
		endPoint := rangePoint{value: types.NewDatum(v.GetValue())}
		rangePoints = append(rangePoints, startPoint, endPoint)
	}
	sorter := rangePointSorter{points: rangePoints}
	sort.Sort(&sorter)
	if sorter.err != nil {
		r.err = sorter.err
	}
	// check duplicates
	hasDuplicate := false
	isStart := false
	for _, v := range rangePoints {
		if isStart == v.start {
			hasDuplicate = true
			break
		}
		isStart = v.start
	}
	if !hasDuplicate {
		return rangePoints
	}
	// remove duplicates
	distinctRangePoints := make([]rangePoint, 0, len(rangePoints))
	isStart = false
	for i := 0; i < len(rangePoints); i++ {
		current := rangePoints[i]
		if isStart == current.start {
			continue
		}
		distinctRangePoints = append(distinctRangePoints, current)
		isStart = current.start
	}
	return distinctRangePoints
}

func (r *rangeBuilder) buildFromBetween(x *ast.BetweenExpr) []rangePoint {
	if x.Not {
		binop1 := &ast.BinaryOperationExpr{Op: opcode.LT, L: x.Expr, R: x.Left}
		binop2 := &ast.BinaryOperationExpr{Op: opcode.GT, L: x.Expr, R: x.Right}
		range1 := r.buildFromBinop(binop1)
		range2 := r.buildFromBinop(binop2)
		return r.union(range1, range2)
	}
	binop1 := &ast.BinaryOperationExpr{Op: opcode.GE, L: x.Expr, R: x.Left}
	binop2 := &ast.BinaryOperationExpr{Op: opcode.LE, L: x.Expr, R: x.Right}
	range1 := r.buildFromBinop(binop1)
	range2 := r.buildFromBinop(binop2)
	return r.intersection(range1, range2)
}

func (r *rangeBuilder) buildFromIsNull(x *ast.IsNullExpr) []rangePoint {
	if x.Not {
		startPoint := rangePoint{value: types.MinNotNullDatum(), start: true}
		endPoint := rangePoint{value: types.MaxValueDatum()}
		return []rangePoint{startPoint, endPoint}
	}
	startPoint := rangePoint{start: true}
	endPoint := rangePoint{}
	return []rangePoint{startPoint, endPoint}
}

func (r *rangeBuilder) buildFromIsTruth(x *ast.IsTruthExpr) []rangePoint {
	if x.True != 0 {
		if x.Not {
			// NOT TRUE range is {[null null] [0, 0]}
			startPoint1 := rangePoint{start: true}
			endPoint1 := rangePoint{}
			startPoint2 := rangePoint{start: true}
			startPoint2.value.SetInt64(0)
			endPoint2 := rangePoint{}
			endPoint2.value.SetInt64(0)
			return []rangePoint{startPoint1, endPoint1, startPoint2, endPoint2}
		}
		// TRUE range is {[-inf 0) (0 +inf]}
		startPoint1 := rangePoint{value: types.MinNotNullDatum(), start: true}
		endPoint1 := rangePoint{excl: true}
		endPoint1.value.SetInt64(0)
		startPoint2 := rangePoint{excl: true, start: true}
		startPoint2.value.SetInt64(0)
		endPoint2 := rangePoint{value: types.MaxValueDatum()}
		return []rangePoint{startPoint1, endPoint1, startPoint2, endPoint2}
	}
	if x.Not {
		startPoint1 := rangePoint{start: true}
		endPoint1 := rangePoint{excl: true}
		endPoint1.value.SetInt64(0)
		startPoint2 := rangePoint{start: true, excl: true}
		startPoint2.value.SetInt64(0)
		endPoint2 := rangePoint{value: types.MaxValueDatum()}
		return []rangePoint{startPoint1, endPoint1, startPoint2, endPoint2}
	}
	startPoint := rangePoint{start: true}
	startPoint.value.SetInt64(0)
	endPoint := rangePoint{}
	endPoint.value.SetInt64(0)
	return []rangePoint{startPoint, endPoint}
}

func (r *rangeBuilder) buildFromPatternLike(x *ast.PatternLikeExpr) []rangePoint {
	if x.Not {
		// Pattern not like is not supported.
		r.err = ErrUnsupportedType.Gen("NOT LIKE is not supported.")
		return fullRange
	}
	pattern, err := types.ToString(x.Pattern.GetValue())
	if err != nil {
		r.err = errors.Trace(err)
		return fullRange
	}
	lowValue := make([]byte, 0, len(pattern))
	// unscape the pattern
	var exclude bool
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == x.Escape {
			i++
			if i < len(pattern) {
				lowValue = append(lowValue, pattern[i])
			} else {
				lowValue = append(lowValue, x.Escape)
			}
			continue
		}
		if pattern[i] == '%' {
			break
		} else if pattern[i] == '_' {
			exclude = true
			break
		}
		lowValue = append(lowValue, pattern[i])
	}
	if len(lowValue) == 0 {
		return []rangePoint{{value: types.MinNotNullDatum(), start: true}, {value: types.MaxValueDatum()}}
	}
	startPoint := rangePoint{start: true, excl: exclude}
	startPoint.value.SetBytesAsString(lowValue)
	highValue := make([]byte, len(lowValue))
	copy(highValue, lowValue)
	endPoint := rangePoint{excl: true}
	for i := len(highValue) - 1; i >= 0; i-- {
		highValue[i]++
		if highValue[i] != 0 {
			endPoint.value.SetBytesAsString(highValue)
			break
		}
		if i == 0 {
			endPoint.value = types.MaxValueDatum()
			break
		}
	}
	ranges := make([]rangePoint, 2)
	ranges[0] = startPoint
	ranges[1] = endPoint
	return ranges
}

func (r *rangeBuilder) buildFromColumnName(x *ast.ColumnNameExpr) []rangePoint {
	// column name expression is equivalent to column name is true.
	startPoint1 := rangePoint{value: types.MinNotNullDatum(), start: true}
	endPoint1 := rangePoint{excl: true}
	endPoint1.value.SetInt64(0)
	startPoint2 := rangePoint{excl: true, start: true}
	startPoint2.value.SetInt64(0)
	endPoint2 := rangePoint{value: types.MaxValueDatum()}
	return []rangePoint{startPoint1, endPoint1, startPoint2, endPoint2}
}

func (r *rangeBuilder) intersection(a, b []rangePoint) []rangePoint {
	return r.merge(a, b, false)
}

func (r *rangeBuilder) union(a, b []rangePoint) []rangePoint {
	return r.merge(a, b, true)
}

func (r *rangeBuilder) merge(a, b []rangePoint, union bool) []rangePoint {
	sorter := rangePointSorter{points: append(a, b...)}
	sort.Sort(&sorter)
	if sorter.err != nil {
		r.err = sorter.err
		return nil
	}
	var (
		merged               []rangePoint
		inRangeCount         int
		requiredInRangeCount int
	)
	if union {
		requiredInRangeCount = 1
	} else {
		requiredInRangeCount = 2
	}
	for _, val := range sorter.points {
		if val.start {
			inRangeCount++
			if inRangeCount == requiredInRangeCount {
				// just reached the required in range count, a new range started.
				merged = append(merged, val)
			}
		} else {
			if inRangeCount == requiredInRangeCount {
				// just about to leave the required in range count, the range is ended.
				merged = append(merged, val)
			}
			inRangeCount--
		}
	}
	return merged
}

// buildIndexRanges build index ranges from range points.
// Only the first column in the index is built, extra column ranges will be appended by
// appendIndexRanges.
func (r *rangeBuilder) buildIndexRanges(rangePoints []rangePoint) []*IndexRange {
	indexRanges := make([]*IndexRange, 0, len(rangePoints)/2)
	for i := 0; i < len(rangePoints); i += 2 {
		startPoint := rangePoints[i]
		endPoint := rangePoints[i+1]
		ir := &IndexRange{
			LowVal:      []types.Datum{startPoint.value},
			LowExclude:  startPoint.excl,
			HighVal:     []types.Datum{endPoint.value},
			HighExclude: endPoint.excl,
		}
		indexRanges = append(indexRanges, ir)
	}
	return indexRanges
}

// appendIndexRanges appends additional column ranges for multi-column index.
// The additional column ranges can only be appended to point ranges.
// for example we have an index (a, b), if the condition is (a > 1 and b = 2)
// then we can not build a conjunctive ranges for this index.
func (r *rangeBuilder) appendIndexRanges(origin []*IndexRange, rangePoints []rangePoint) []*IndexRange {
	var newIndexRanges []*IndexRange
	for i := 0; i < len(origin); i++ {
		oRange := origin[i]
		if !oRange.IsPoint() {
			newIndexRanges = append(newIndexRanges, oRange)
		} else {
			newIndexRanges = append(newIndexRanges, r.appendIndexRange(oRange, rangePoints)...)
		}
	}
	return newIndexRanges
}

func (r *rangeBuilder) appendIndexRange(origin *IndexRange, rangePoints []rangePoint) []*IndexRange {
	newRanges := make([]*IndexRange, 0, len(rangePoints)/2)
	for i := 0; i < len(rangePoints); i += 2 {
		startPoint := rangePoints[i]
		lowVal := make([]types.Datum, len(origin.LowVal)+1)
		copy(lowVal, origin.LowVal)
		lowVal[len(origin.LowVal)] = startPoint.value

		endPoint := rangePoints[i+1]
		highVal := make([]types.Datum, len(origin.HighVal)+1)
		copy(highVal, origin.HighVal)
		highVal[len(origin.HighVal)] = endPoint.value

		ir := &IndexRange{
			LowVal:      lowVal,
			LowExclude:  startPoint.excl,
			HighVal:     highVal,
			HighExclude: endPoint.excl,
		}
		newRanges = append(newRanges, ir)
	}
	return newRanges
}

func (r *rangeBuilder) buildTableRanges(rangePoints []rangePoint) []TableRange {
	tableRanges := make([]TableRange, 0, len(rangePoints)/2)
	for i := 0; i < len(rangePoints); i += 2 {
		startPoint := rangePoints[i]
		if startPoint.value.Kind() == types.KindNull || startPoint.value.Kind() == types.KindMinNotNull {
			startPoint.value.SetInt64(math.MinInt64)
		}
		startInt, err := types.ToInt64(startPoint.value.GetValue())
		if err != nil {
			r.err = errors.Trace(err)
			return tableRanges
		}
		startDatum := types.NewDatum(startInt)
		cmp, err := startDatum.CompareDatum(startPoint.value)
		if err != nil {
			r.err = errors.Trace(err)
			return tableRanges
		}
		if cmp < 0 || (cmp == 0 && startPoint.excl) {
			startInt++
		}
		endPoint := rangePoints[i+1]
		if endPoint.value.Kind() == types.KindNull {
			endPoint.value.SetInt64(math.MinInt64)
		} else if endPoint.value.Kind() == types.KindMaxValue {
			endPoint.value.SetInt64(math.MaxInt64)
		}
		endInt, err := types.ToInt64(endPoint.value.GetValue())
		if err != nil {
			r.err = errors.Trace(err)
			return tableRanges
		}
		endDatum := types.NewDatum(endInt)
		cmp, err = endDatum.CompareDatum(endPoint.value)
		if err != nil {
			r.err = errors.Trace(err)
			return tableRanges
		}
		if cmp > 0 || (cmp == 0 && endPoint.excl) {
			endInt--
		}
		if startInt > endInt {
			continue
		}
		tableRanges = append(tableRanges, TableRange{LowVal: startInt, HighVal: endInt})
	}
	return tableRanges
}
