// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
)

type EventWriter interface {
	EventWriterBase
}

type EventWriterProvider func(name string, mode WriterMode) EventWriter

var eventWriterProviders = map[string]EventWriterProvider{}

func RegisterEventWriter(writerType string, p EventWriterProvider) {
	eventWriterProviders[writerType] = p
}

func HasEventWriter(writerType string) bool {
	_, ok := eventWriterProviders[writerType]
	return ok
}

type WriterMode struct {
	WriterType string
	// ModeName   string

	BufferLen int

	Level Level

	Prefix   string
	Colorize bool
	Flags    int

	Expression string

	StacktraceLevel Level

	WriterOption any
}

func NewEventWriter(name string, mode WriterMode) (EventWriter, error) {
	if p, ok := eventWriterProviders[mode.WriterType]; ok {
		return p(name, mode), nil
	}
	return nil, fmt.Errorf("unknown event writer type %q for writer %q", mode.WriterType, name)
}
