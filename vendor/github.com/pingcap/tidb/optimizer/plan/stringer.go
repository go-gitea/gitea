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
	"strings"
)

// ToString explains a Plan, returns description string.
func ToString(p Plan) string {
	var e stringer
	p.Accept(&e)
	return strings.Join(e.strs, "->")
}

type stringer struct {
	strs []string
	idxs []int
}

func (e *stringer) Enter(in Plan) (Plan, bool) {
	switch in.(type) {
	case *JoinOuter, *JoinInner:
		e.idxs = append(e.idxs, len(e.strs))
	}
	return in, false
}

func (e *stringer) Leave(in Plan) (Plan, bool) {
	var str string
	switch x := in.(type) {
	case *CheckTable:
		str = "CheckTable"
	case *IndexScan:
		str = fmt.Sprintf("Index(%s.%s)", x.Table.Name.L, x.Index.Name.L)
	case *Limit:
		str = "Limit"
	case *SelectFields:
		str = "Fields"
	case *SelectLock:
		str = "Lock"
	case *ShowDDL:
		str = "ShowDDL"
	case *Sort:
		str = "Sort"
	case *TableScan:
		if len(x.Ranges) > 0 {
			ran := x.Ranges[0]
			if ran.LowVal != math.MinInt64 || ran.HighVal != math.MaxInt64 {
				str = fmt.Sprintf("Range(%s)", x.Table.Name.L)
			} else {
				str = fmt.Sprintf("Table(%s)", x.Table.Name.L)
			}
		} else {
			str = fmt.Sprintf("Table(%s)", x.Table.Name.L)
		}
	case *JoinOuter:
		last := len(e.idxs) - 1
		idx := e.idxs[last]
		chilrden := e.strs[idx:]
		e.strs = e.strs[:idx]
		str = "OuterJoin{" + strings.Join(chilrden, "->") + "}"
		e.idxs = e.idxs[:last]
	case *JoinInner:
		last := len(e.idxs) - 1
		idx := e.idxs[last]
		chilrden := e.strs[idx:]
		e.strs = e.strs[:idx]
		str = "InnerJoin{" + strings.Join(chilrden, "->") + "}"
		e.idxs = e.idxs[:last]
	default:
		str = fmt.Sprintf("%T", in)
	}
	e.strs = append(e.strs, str)
	return in, true
}
