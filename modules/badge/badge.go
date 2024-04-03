// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package badge

import (
	actions_model "code.gitea.io/gitea/models/actions"
)

// The Badge layout: |offset|label|message|
// We use 10x scale to calculate more precisely
// Then scale down to normal size in tmpl file

type Label struct {
	text  string
	width int
}

func (l Label) Text() string {
	return l.text
}

func (l Label) Width() int {
	return l.width
}

func (l Label) TextLength() int {
	return int(float64(l.width-defaultOffset) * 9.5)
}

func (l Label) X() int {
	return l.width*5 + 10
}

type Message struct {
	text  string
	width int
	x     int
}

func (m Message) Text() string {
	return m.text
}

func (m Message) Width() int {
	return m.width
}

func (m Message) X() int {
	return m.x
}

func (m Message) TextLength() int {
	return int(float64(m.width-defaultOffset) * 9.5)
}

type Badge struct {
	Color    string
	FontSize int
	Label    Label
	Message  Message
}

func (b Badge) Width() int {
	return b.Label.width + b.Message.width
}

const (
	defaultOffset    = 9
	defaultFontSize  = 11
	DefaultColor     = "#9f9f9f" // Grey
	defaultFontWidth = 7         // approximate speculation
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
	lw := defaultFontWidth*len(label) + defaultOffset
	mw := defaultFontWidth*len(message) + defaultOffset
	x := lw*10 + mw*5 - 10
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
