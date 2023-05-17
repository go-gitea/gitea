// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
)

type EventWriter interface {
	EventWriterBase
}

type EventWriterProvider func(writerName string, writerMode WriterMode) EventWriter

var eventWriterProviders = map[string]EventWriterProvider{}

func RegisterEventWriter(writerType string, p EventWriterProvider) {
	eventWriterProviders[writerType] = p
}

func HasEventWriter(writerType string) bool {
	_, ok := eventWriterProviders[writerType]
	return ok
}

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

func NewEventWriter(name, writerType string, mode WriterMode) (EventWriter, error) {
	if p, ok := eventWriterProviders[writerType]; ok {
		return p(name, mode), nil
	}
	return nil, fmt.Errorf("unknown event writer type %q for writer %q", writerType, name)
}
