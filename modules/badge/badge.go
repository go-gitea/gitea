// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package badge

import (
	"strings"
	"unicode"

	actions_model "code.gitea.io/gitea/models/actions"
)

// The Badge layout: |offset|label|message|
// We use 10x scale to calculate more precisely
// Then scale down to normal size in tmpl file

type Text struct {
	text  string
	width int
	x     int
}

func (t Text) Text() string {
	return t.text
}

func (t Text) Width() int {
	return t.width
}

func (t Text) X() int {
	return t.x
}

func (t Text) TextLength() int {
	return int(float64(t.width-defaultOffset) * 10)
}

type Badge struct {
	IDPrefix   string
	FontFamily string
	Color      string
	FontSize   int
	Label      Text
	Message    Text
}

func (b Badge) Width() int {
	return b.Label.width + b.Message.width
}

const (
	defaultOffset     = 10
	defaultFontSize   = 11
	DefaultColor      = "#9f9f9f" // Grey
	DefaultFontFamily = "DejaVu Sans,Verdana,Geneva,sans-serif"
)

var StatusColorMap = map[actions_model.Status]string{
	actions_model.StatusSuccess:   "#4c1",    // Green
	actions_model.StatusSkipped:   "#dfb317", // Yellow
	actions_model.StatusUnknown:   "#97ca00", // Light Green
	actions_model.StatusFailure:   "#e05d44", // Red
	actions_model.StatusCancelled: "#fe7d37", // Orange
	actions_model.StatusWaiting:   "#dfb317", // Yellow
	actions_model.StatusRunning:   "#dfb317", // Yellow
	actions_model.StatusBlocked:   "#dfb317", // Yellow
}

// GenerateBadge generates badge with given template
func GenerateBadge(label, message, color string) Badge {
	lw := calculateTextWidth(label) + defaultOffset
	mw := calculateTextWidth(message) + defaultOffset

	lx := lw * 5
	mx := lw*10 + mw*5 - 10
	return Badge{
		FontFamily: DefaultFontFamily,
		Label: Text{
			text:  label,
			width: lw,
			x:     lx,
		},
		Message: Text{
			text:  message,
			width: mw,
			x:     mx,
		},
		FontSize: defaultFontSize * 10,
		Color:    color,
	}
}

func calculateTextWidth(text string) int {
	width := 0
	widthData := DejaVuGlyphWidthData()
	for _, char := range strings.TrimSpace(text) {
		charWidth, ok := widthData[char]
		if !ok {
			// use the width of 'm' in case of missing glyph width data for a printable character
			if unicode.IsPrint(char) {
				charWidth = widthData['m']
			} else {
				charWidth = 0
			}
		}
		width += int(charWidth)
	}

	return width
}
