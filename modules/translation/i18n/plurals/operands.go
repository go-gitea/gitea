// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package plurals

import (
	"fmt"
	"strconv"
	"strings"
)

func intInRange(i, from, to int64) bool {
	return from <= i && i <= to
}

func intEqualsAny(i int64, any ...int64) bool {
	for _, a := range any {
		if i == a {
			return true
		}
	}
	return false
}

// Operands is a representation of http://unicode.org/reports/tr35/tr35-numbers.html#Operands
type Operands struct {
	N float64 // absolute value of the source number (integer and decimals)
	I int64   // integer digits of n
	E int64   // exponent
	V int64   // number of visible fraction digits in n, with trailing zeros
	W int64   // number of visible fraction digits in n, without trailing zeros
	F int64   // visible fractional digits in n, with trailing zeros
	T int64   // visible fractional digits in n, without trailing zeros
}

// NEqualsAny returns true if o represents an integer equal to any of the arguments.
func (o *Operands) NEqualsAny(any ...int64) bool {
	if o.T != 0 {
		return false
	}

	return intEqualsAny(o.I, any...)
}

// NModEqualsAny returns true if o represents an integer equal to any of the arguments modulo mod.
func (o *Operands) NModEqualsAny(mod int64, any ...int64) bool {
	if o.T != 0 {
		return false
	}

	modI := o.I % mod
	return intEqualsAny(modI, any...)
}

// NInRange returns true if o represents an integer in the closed interval [from, to].
func (o *Operands) NInRange(from, to int64) bool {
	return o.T == 0 && intInRange(o.I, from, to)
}

// NModInRange returns true if o represents an integer in the closed interval [from, to] modulo mod.
func (o *Operands) NModInRange(mod, from, to int64) bool {
	modI := o.I % mod
	return o.T == 0 && intInRange(modI, from, to)
}

// NewOperands returns the operands for number.
func NewOperands(number interface{}) (*Operands, error) {
	switch number := number.(type) {
	case int:
		return operandsFromInt64(int64(number)), nil
	case int8:
		return operandsFromInt64(int64(number)), nil
	case int16:
		return operandsFromInt64(int64(number)), nil
	case int32:
		return operandsFromInt64(int64(number)), nil
	case int64:
		return operandsFromInt64(number), nil
	case string:
		return operandsFromString(number)
	case float32, float64:
		return nil, fmt.Errorf("floats must be formatted as a string")
	default:
		return nil, fmt.Errorf("invalid type %T; expected integer or string", number)
	}
}

func operandsFromInt64(i int64) *Operands {
	if i < 0 {
		i = -i
	}
	return &Operands{float64(i), i, 0, 0, 0, 0, 0}
}

func operandsFromString(s string) (*Operands, error) {
	s = strings.TrimSpace(s)

	// strip the sign
	if s[0] == '-' {
		s = s[1:]
	}

	ops := &Operands{}

	// Now the problem is s could be in [1-9](.[0-9]+)?e[1-9][0-9]*
	// We need to determine how many numbers after the decimal place remain.
	s = strings.Replace(s, "e", "c", 1)
	if parts := strings.SplitN(s, "c", 2); len(parts) == 2 {
		if idx := strings.Index(parts[0], "."); idx >= 0 {
			numberOfDecimalsPreExp := len(parts[0]) - idx - 1
			exp, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, err
			}
			ops.E = int64(exp)
			if exp >= numberOfDecimalsPreExp {
				s = parts[0][:idx] + parts[0][idx+1:]
				exp -= numberOfDecimalsPreExp
				s += strings.Repeat("0", exp)
			} else {
				s = parts[0][:idx] + parts[0][idx+1:len(parts[0])+exp-numberOfDecimalsPreExp] + "." + parts[0][len(parts[0])+exp-numberOfDecimalsPreExp:]
			}
		} else {
			exp, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, err
			}
			ops.E = int64(exp)

			s = parts[0] + strings.Repeat("0", exp)
		}
	}

	// attempt to parse as a float
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}

	// ops.N is the value of the number
	ops.N = n

	// Now split at the "."
	parts := strings.SplitN(s, ".", 2)

	// ops.I is the integer floor of the number
	ops.I, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}

	// if there is no decimal part the rest of the parts of the operand is 0
	if len(parts) == 1 {
		return ops, nil
	}

	// parts[1] is the decimal part
	fraction := parts[1]

	// V is the number of visible fraction digits in n, with trailing zeros
	ops.V = int64(len(fraction))
	for i := ops.V - 1; i >= 0; i-- {
		if fraction[i] != '0' {
			// W is the number of visible fraction digits in n, without trailing zeros
			ops.W = i + 1
			break
		}
	}

	if ops.V > 0 {
		// F is the visible fractional digits in n, with trailing zeros
		// we get this from the V
		f, err := strconv.ParseInt(fraction, 10, 0)
		if err != nil {
			return nil, err
		}
		ops.F = f
	}
	if ops.W > 0 {
		// T is visible fractional digits in n, without trailing zeros
		// we get this from the W
		t, err := strconv.ParseInt(fraction[:ops.W], 10, 0)
		if err != nil {
			return nil, err
		}
		ops.T = t
	}
	return ops, nil
}
