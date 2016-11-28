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

import (
	"fmt"
	"github.com/pingcap/tidb/util/types"
)

// ToString converts a node to a string for debugging purpose.
func ToString(node Node) string {
	s := &stringer{strMap: map[Node]string{}}
	node.Accept(s)
	return s.strMap[node]
}

type stringer struct {
	strMap map[Node]string
}

// Enter implements Visitor Enter interface.
func (c *stringer) Enter(node Node) (Node, bool) {
	return node, false
}

// Leave implements Visitor Leave interface.
func (c *stringer) Leave(in Node) (out Node, ok bool) {
	switch x := in.(type) {
	case *BinaryOperationExpr:
		left := c.strMap[x.L]
		right := c.strMap[x.R]
		c.strMap[x] = left + " " + x.Op.String() + " " + right
	case *ValueExpr:
		str, _ := types.ToString(x.GetValue())
		c.strMap[x] = str
	case *ParenthesesExpr:
		c.strMap[x] = "(" + c.strMap[x.Expr] + ")"
	case *ColumnNameExpr:
		c.strMap[x] = x.Name.Table.O + "." + x.Name.Name.O
	case *BetweenExpr:
		c.strMap[x] = c.strMap[x.Expr] + " BETWWEN " + c.strMap[x.Left] + " AND " + c.strMap[x.Right]
	default:
		c.strMap[in] = fmt.Sprintf("%T", in)
	}
	return in, true
}
