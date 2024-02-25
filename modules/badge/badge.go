// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package badge

import (
	actions_model "code.gitea.io/gitea/models/actions"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

type Label struct {
	text  string
	width float64
}

func (l Label) Text() string {
	return l.text
}

func (l Label) Width() int {
	return int(l.width)
}

func (l Label) TextLength() int {
	return int((l.width - defaultOffset) * 9.5)
}

func (l Label) X() int {
	return int((l.width/2 + 1) * 10)
}

type Message struct {
	text  string
	width float64
	x     int
}

func (m Message) Text() string {
	return m.text
}

func (m Message) Width() int {
	return int(m.width)
}

func (m Message) X() int {
	return m.x
}

func (m Message) TextLength() int {
	return int((m.width - defaultOffset) * 9.5)
}

type Badge struct {
	Color    string
	FontSize int
	Label    Label
	Message  Message
}

func (b Badge) Width() int {
	return int(b.Label.width + b.Message.width)
}

const (
	defaultOffset   = float64(9)
	defaultFontSize = 11
	DefaultColor    = "#9f9f9f" // Grey
)

var drawer = &font.Drawer{
	Face: basicfont.Face7x13,
}

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
	lw := float64(drawer.MeasureString(label)>>6) + defaultOffset
	mw := float64(drawer.MeasureString(message)>>6) + defaultOffset
	x := int((lw + (mw / 2) - 1) * 10)
	return Badge{
		Label: Label{
			text:  label,
			width: lw,
		},
		Message: Message{
			text:  message,
			width: mw,
			x:     x,
		},
		FontSize: defaultFontSize * 10,
		Color:    color,
	}
}
