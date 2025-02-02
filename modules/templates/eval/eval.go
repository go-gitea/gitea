// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package eval

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

type Num struct {
	Value any // int64 or float64, nil on error
}

var opPrecedence = map[string]int{
	// "(": 1, this is for low precedence like function calls, they are handled separately
	"or":  2,
	"and": 3,
	"not": 4,
	"==":  5, "!=": 5, "<": 5, "<=": 5, ">": 5, ">=": 5,
	"+": 6, "-": 6,
	"*": 7, "/": 7,
}

type stack[T any] struct {
	name  string
	elems []T
}

func (s *stack[T]) push(t T) {
	s.elems = append(s.elems, t)
}

func (s *stack[T]) pop() T {
	if len(s.elems) == 0 {
		panic(s.name + " stack is empty")
	}
	t := s.elems[len(s.elems)-1]
	s.elems = s.elems[:len(s.elems)-1]
	return t
}

func (s *stack[T]) peek() T {
	if len(s.elems) == 0 {
		panic(s.name + " stack is empty")
	}
	return s.elems[len(s.elems)-1]
}

type operator string

type eval struct {
	stackNum stack[Num]
	stackOp  stack[operator]
	funcMap  map[string]func([]Num) Num
}

func newEval() *eval {
	e := &eval{}
	e.stackNum.name = "num"
	e.stackOp.name = "op"
	return e
}

func toNum(v any) (Num, error) {
	switch v := v.(type) {
	case string:
		if strings.Contains(v, ".") {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return Num{n}, err
			}
			return Num{n}, nil
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Num{n}, err
		}
		return Num{n}, nil
	case float32, float64:
		n, _ := util.ToFloat64(v)
		return Num{n}, nil
	default:
		n, err := util.ToInt64(v)
		if err != nil {
			return Num{n}, err
		}
		return Num{n}, nil
	}
}

func truth(b bool) int64 {
	if b {
		return int64(1)
	}
	return int64(0)
}

func applyOp2Generic[T int64 | float64](op operator, n1, n2 T) Num {
	switch op {
	case "+":
		return Num{n1 + n2}
	case "-":
		return Num{n1 - n2}
	case "*":
		return Num{n1 * n2}
	case "/":
		return Num{n1 / n2}
	case "==":
		return Num{truth(n1 == n2)}
	case "!=":
		return Num{truth(n1 != n2)}
	case "<":
		return Num{truth(n1 < n2)}
	case "<=":
		return Num{truth(n1 <= n2)}
	case ">":
		return Num{truth(n1 > n2)}
	case ">=":
		return Num{truth(n1 >= n2)}
	case "and":
		t1, _ := util.ToFloat64(n1)
		t2, _ := util.ToFloat64(n2)
		return Num{truth(t1 != 0 && t2 != 0)}
	case "or":
		t1, _ := util.ToFloat64(n1)
		t2, _ := util.ToFloat64(n2)
		return Num{truth(t1 != 0 || t2 != 0)}
	}
	panic("unknown operator: " + string(op))
}

func applyOp2(op operator, n1, n2 Num) Num {
	float := false
	if _, ok := n1.Value.(float64); ok {
		float = true
	} else if _, ok = n2.Value.(float64); ok {
		float = true
	}
	if float {
		f1, _ := util.ToFloat64(n1.Value)
		f2, _ := util.ToFloat64(n2.Value)
		return applyOp2Generic(op, f1, f2)
	}
	return applyOp2Generic(op, n1.Value.(int64), n2.Value.(int64))
}

func toOp(v any) (operator, error) {
	if v, ok := v.(string); ok {
		return operator(v), nil
	}
	return "", fmt.Errorf(`unsupported token type "%T"`, v)
}

func (op operator) hasOpenBracket() bool {
	return strings.HasSuffix(string(op), "(") // it's used to support functions like "sum("
}

func (op operator) isComma() bool {
	return op == ","
}

func (op operator) isCloseBracket() bool {
	return op == ")"
}

type ExprError struct {
	msg    string
	tokens []any
	err    error
}

func (err ExprError) Error() string {
	sb := strings.Builder{}
	sb.WriteString(err.msg)
	sb.WriteString(" [ ")
	for _, token := range err.tokens {
		_, _ = fmt.Fprintf(&sb, `"%v" `, token)
	}
	sb.WriteString("]")
	if err.err != nil {
		sb.WriteString(": ")
		sb.WriteString(err.err.Error())
	}
	return sb.String()
}

func (err ExprError) Unwrap() error {
	return err.err
}

func (e *eval) applyOp() {
	op := e.stackOp.pop()
	if op == "not" {
		num := e.stackNum.pop()
		i, _ := util.ToInt64(num.Value)
		e.stackNum.push(Num{truth(i == 0)})
	} else if op.hasOpenBracket() || op.isCloseBracket() || op.isComma() {
		panic(fmt.Sprintf("incomplete sub-expression with operator %q", op))
	} else {
		num2 := e.stackNum.pop()
		num1 := e.stackNum.pop()
		e.stackNum.push(applyOp2(op, num1, num2))
	}
}

func (e *eval) exec(tokens ...any) (ret Num, err error) {
	defer func() {
		if r := recover(); r != nil {
			rErr, ok := r.(error)
			if !ok {
				rErr = fmt.Errorf("%v", r)
			}
			err = ExprError{"invalid expression", tokens, rErr}
		}
	}()
	for _, token := range tokens {
		n, err := toNum(token)
		if err == nil {
			e.stackNum.push(n)
			continue
		}

		op, err := toOp(token)
		if err != nil {
			return Num{}, ExprError{"invalid expression", tokens, err}
		}

		switch {
		case op.hasOpenBracket():
			e.stackOp.push(op)
		case op.isCloseBracket(), op.isComma():
			var stackTopOp operator
			for len(e.stackOp.elems) > 0 {
				stackTopOp = e.stackOp.peek()
				if stackTopOp.hasOpenBracket() || stackTopOp.isComma() {
					break
				}
				e.applyOp()
			}
			if op.isCloseBracket() {
				nums := []Num{e.stackNum.pop()}
				for !e.stackOp.peek().hasOpenBracket() {
					stackTopOp = e.stackOp.pop()
					if !stackTopOp.isComma() {
						return Num{}, ExprError{"bracket doesn't match", tokens, nil}
					}
					nums = append(nums, e.stackNum.pop())
				}
				for i, j := 0, len(nums)-1; i < j; i, j = i+1, j-1 {
					nums[i], nums[j] = nums[j], nums[i] // reverse nums slice, to get the right order for arguments
				}
				stackTopOp = e.stackOp.pop()
				fn := string(stackTopOp[:len(stackTopOp)-1])
				if fn == "" {
					if len(nums) != 1 {
						return Num{}, ExprError{"too many values in one bracket", tokens, nil}
					}
					e.stackNum.push(nums[0])
				} else if f, ok := e.funcMap[fn]; ok {
					e.stackNum.push(f(nums))
				} else {
					return Num{}, ExprError{"unknown function: " + fn, tokens, nil}
				}
			} else {
				e.stackOp.push(op)
			}
		default:
			for len(e.stackOp.elems) > 0 && len(e.stackNum.elems) > 0 {
				stackTopOp := e.stackOp.peek()
				if stackTopOp.hasOpenBracket() || stackTopOp.isComma() || precedence(stackTopOp, op) < 0 {
					break
				}
				e.applyOp()
			}
			e.stackOp.push(op)
		}
	}
	for len(e.stackOp.elems) > 0 && !e.stackOp.peek().isComma() {
		e.applyOp()
	}
	if len(e.stackNum.elems) != 1 {
		return Num{}, ExprError{fmt.Sprintf("expect 1 value as final result, but there are %d", len(e.stackNum.elems)), tokens, nil}
	}
	return e.stackNum.pop(), nil
}

func precedence(op1, op2 operator) int {
	p1 := opPrecedence[string(op1)]
	p2 := opPrecedence[string(op2)]
	if p1 == 0 {
		panic("unknown operator precedence: " + string(op1))
	} else if p2 == 0 {
		panic("unknown operator precedence: " + string(op2))
	}
	return p1 - p2
}

func castFloat64(nums []Num) bool {
	hasFloat := false
	for _, num := range nums {
		if _, hasFloat = num.Value.(float64); hasFloat {
			break
		}
	}
	if hasFloat {
		for i, num := range nums {
			if _, ok := num.Value.(float64); !ok {
				f, _ := util.ToFloat64(num.Value)
				nums[i] = Num{f}
			}
		}
	}
	return hasFloat
}

func fnSum(nums []Num) Num {
	if castFloat64(nums) {
		var sum float64
		for _, num := range nums {
			sum += num.Value.(float64)
		}
		return Num{sum}
	}
	var sum int64
	for _, num := range nums {
		sum += num.Value.(int64)
	}
	return Num{sum}
}

// Expr evaluates the given expression tokens and returns the result.
// It supports the following operators: +, -, *, /, and, or, not, ==, !=, >, >=, <, <=.
// Non-zero values are treated as true, zero values are treated as false.
// If no error occurs, the result is either an int64 or a float64.
// If all numbers are integer, the result is an int64, otherwise if there is any float number, the result is a float64.
func Expr(tokens ...any) (Num, error) {
	e := newEval()
	e.funcMap = map[string]func([]Num) Num{"sum": fnSum}
	return e.exec(tokens...)
}
