// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"strconv"
)

const escape = "\033"

// ColorAttribute defines a single SGR Code
type ColorAttribute int

// Base ColorAttributes
const (
	Reset ColorAttribute = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground text colors
const (
	FgBlack ColorAttribute = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Foreground Hi-Intensity text colors
const (
	FgHiBlack ColorAttribute = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background text colors
const (
	BgBlack ColorAttribute = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
)

// Background Hi-Intensity text colors
const (
	BgHiBlack ColorAttribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

var (
	resetBytes   = ColorBytes(Reset)
	fgCyanBytes  = ColorBytes(FgCyan)
	fgGreenBytes = ColorBytes(FgGreen)
)

type ColoredValue struct {
	v      any
	colors []ColorAttribute
}

var _ fmt.Formatter = (*ColoredValue)(nil)

func (c *ColoredValue) Format(f fmt.State, verb rune) {
	_, _ = f.Write(ColorBytes(c.colors...))
	s := fmt.Sprintf(fmt.FormatString(f, verb), c.v)
	_, _ = f.Write([]byte(s))
	_, _ = f.Write(resetBytes)
}

func (c *ColoredValue) Value() any {
	return c.v
}

func NewColoredValue(v any, color ...ColorAttribute) *ColoredValue {
	return &ColoredValue{v: v, colors: color}
}

// ColorBytes converts a list of ColorAttributes to a byte array
func ColorBytes(attrs ...ColorAttribute) []byte {
	bytes := make([]byte, 0, 20)
	bytes = append(bytes, escape[0], '[')
	if len(attrs) > 0 {
		bytes = append(bytes, strconv.Itoa(int(attrs[0]))...)
		for _, a := range attrs[1:] {
			bytes = append(bytes, ';')
			bytes = append(bytes, strconv.Itoa(int(a))...)
		}
	} else {
		bytes = append(bytes, strconv.Itoa(int(Bold))...)
	}
	bytes = append(bytes, 'm')
	return bytes
}
