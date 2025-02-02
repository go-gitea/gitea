// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
)

// EventWriter is the general interface for all event writers
// EventWriterBase is only used as its base interface
// A writer implementation could override the default EventWriterBase functions
// eg: a writer can override the Run to handle events in its own way with its own goroutine
type EventWriter interface {
	EventWriterBase
}

// WriterMode is the mode for creating a new EventWriter, it contains common options for all writers
// Its WriterOption field is the specified options for a writer, it should be passed by value but not by pointer
type WriterMode struct {
	BufferLen int

	Level    Level
	Prefix   string
	Colorize bool
	Flags    Flags

	Expression string

	StacktraceLevel Level

	WriterOption any
}

// EventWriterProvider is the function for creating a new EventWriter
type EventWriterProvider func(writerName string, writerMode WriterMode) EventWriter

var eventWriterProviders = map[string]EventWriterProvider{}

func RegisterEventWriter(writerType string, p EventWriterProvider) {
	eventWriterProviders[writerType] = p
}

func HasEventWriter(writerType string) bool {
	_, ok := eventWriterProviders[writerType]
	return ok
}

func NewEventWriter(name, writerType string, mode WriterMode) (EventWriter, error) {
	if p, ok := eventWriterProviders[writerType]; ok {
		return p(name, mode), nil
	}
	return nil, fmt.Errorf("unknown event writer type %q for writer %q", writerType, name)
}
