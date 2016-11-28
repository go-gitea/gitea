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

package opcode

import "fmt"

// Op is opcode type.
type Op int

// List operators.
const (
	AndAnd Op = iota + 1
	LeftShift
	RightShift
	OrOr
	GE
	LE
	EQ
	NE
	LT
	GT
	Plus
	Minus
	And
	Or
	Mod
	Xor
	Div
	Mul
	Not
	BitNeg
	IntDiv
	LogicXor
	NullEQ
)

var ops = map[Op]string{
	AndAnd:     "&&",
	LeftShift:  "<<",
	RightShift: ">>",
	OrOr:       "||",
	GE:         ">=",
	LE:         "<=",
	EQ:         "=",
	NE:         "!=",
	LT:         "<",
	GT:         ">",
	Plus:       "+",
	Minus:      "-",
	And:        "&",
	Or:         "|",
	Mod:        "%",
	Xor:        "^",
	Div:        "/",
	Mul:        "*",
	Not:        "!",
	BitNeg:     "~",
	IntDiv:     "DIV",
	LogicXor:   "XOR",
	NullEQ:     "<=>",
}

// String implements Stringer interface.
func (o Op) String() string {
	str, ok := ops[o]
	if !ok {
		panic(fmt.Sprintf("%d", o))
	}

	return str
}
