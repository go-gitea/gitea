// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"strings"
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

var _ColorAttributeToString = map[ColorAttribute]string{
	Reset:        "Reset",
	Bold:         "Bold",
	Faint:        "Faint",
	Italic:       "Italic",
	Underline:    "Underline",
	BlinkSlow:    "BlinkSlow",
	BlinkRapid:   "BlinkRapid",
	ReverseVideo: "ReverseVideo",
	Concealed:    "Concealed",
	CrossedOut:   "CrossedOut",
	FgBlack:      "FgBlack",
	FgRed:        "FgRed",
	FgGreen:      "FgGreen",
	FgYellow:     "FgYellow",
	FgBlue:       "FgBlue",
	FgMagenta:    "FgMagenta",
	FgCyan:       "FgCyan",
	FgWhite:      "FgWhite",
	FgHiBlack:    "FgHiBlack",
	FgHiRed:      "FgHiRed",
	FgHiGreen:    "FgHiGreen",
	FgHiYellow:   "FgHiYellow",
	FgHiBlue:     "FgHiBlue",
	FgHiMagenta:  "FgHiMagenta",
	FgHiCyan:     "FgHiCyan",
	FgHiWhite:    "FgHiWhite",
	BgBlack:      "BgBlack",
	BgRed:        "BgRed",
	BgGreen:      "BgGreen",
	BgYellow:     "BgYellow",
	BgBlue:       "BgBlue",
	BgMagenta:    "BgMagenta",
	BgCyan:       "BgCyan",
	BgWhite:      "BgWhite",
	BgHiBlack:    "BgHiBlack",
	BgHiRed:      "BgHiRed",
	BgHiGreen:    "BgHiGreen",
	BgHiYellow:   "BgHiYellow",
	BgHiBlue:     "BgHiBlue",
	BgHiMagenta:  "BgHiMagenta",
	BgHiCyan:     "BgHiCyan",
	BgHiWhite:    "BgHiWhite",
}

func (c *ColorAttribute) String() string {
	return _ColorAttributeToString[*c]
}

var _ColorAttributeFromString = map[string]ColorAttribute{}

// ColorAttributeFromString will return a ColorAttribute given a string
func ColorAttributeFromString(from string) ColorAttribute {
	lowerFrom := strings.TrimSpace(strings.ToLower(from))
	return _ColorAttributeFromString[lowerFrom]
}

// ColorString converts a list of ColorAttributes to a color string
func ColorString(attrs ...ColorAttribute) string {
	format := make([]string, len(attrs))
	for i, a := range attrs {
		format[i] = fmt.Sprintf("%d", a)
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
var fgGreenString = ColorString(FgGreen)

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

func init() {
	for attr, from := range _ColorAttributeToString {
		_ColorAttributeFromString[strings.ToLower(from)] = attr
	}
}
