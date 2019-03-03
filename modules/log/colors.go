// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"strings"
)

const escape = "\033"

// Attribute defines a single SGR Code
type Attribute int

func (a Attribute) String() string {
	return fmt.Sprintf("%d", a)
}

// Base attributes
const (
	Reset Attribute = iota
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
	FgBlack Attribute = iota + 30
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
	FgHiBlack Attribute = iota + 90
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
	BgBlack Attribute = iota + 40
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
	BgHiBlack Attribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

// ColorString converts a list of attributes to a color string
func ColorString(attrs ...Attribute) string {
	format := make([]string, len(attrs))
	for i, a := range attrs {
		format[i] = a.String()
	}
	return fmt.Sprintf("%s[%sm", escape, strings.Join(format, ";"))
}

var levelToColor = map[Level]string{
	TRACE:    ColorString(Bold, FgCyan),
	DEBUG:    ColorString(Bold, FgBlue),
	INFO:     ColorString(Bold, FgGreen),
	WARN:     ColorString(Bold, FgYellow),
	ERROR:    ColorString(Bold, FgRed),
	CRITICAL: ColorString(Bold, BgMagenta),
	FATAL:    ColorString(Bold, BgRed),
	NONE:     ColorString(Reset),
}

var resetString = ColorString(Reset)
var fgCyanString = ColorString(FgCyan)
var fgWhiteString = ColorString(FgWhite)

var statusToColor = map[int]string{
	200: ColorString(Bold, FgGreen),
	201: ColorString(Bold, FgGreen),
	202: ColorString(Bold, FgGreen),
	301: ColorString(Bold, FgWhite),
	302: ColorString(Bold, FgWhite),
	304: ColorString(Bold, FgYellow),
	401: ColorString(Underline, FgRed),
	403: ColorString(Underline, FgRed),
	404: ColorString(Bold, FgRed),
	500: ColorString(Bold, BgRed),
}
