// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"time"
)

type EventWriterBase interface {
	Base() *EventWriterBaseImpl
	GetWriterType() string
	GetWriterName() string
	GetLevel() Level

	Run(ctx context.Context)
}

type EventWriterBaseImpl struct {
	LoggerImpl *LoggerImpl

	Name  string
	Mode  *WriterMode
	Queue chan *Event

	Formatter         EventFormatter // format the Event to a message and write it to output
	OutputWriteCloser io.WriteCloser // it will be closed when the event writer is stopped

	stopped chan struct{}
}

var _ EventWriterBase = (*EventWriterBaseImpl)(nil)

func (b *EventWriterBaseImpl) Base() *EventWriterBaseImpl {
	return b
}

func (b *EventWriterBaseImpl) GetWriterType() string {
	return b.Mode.WriterType
}

func (b *EventWriterBaseImpl) GetWriterName() string {
	return b.Name
}

func (b *EventWriterBaseImpl) GetLevel() Level {
	return b.Mode.Level
}

func (b *EventWriterBaseImpl) Run(ctx context.Context) {
	defer b.OutputWriteCloser.Close()

	var exprRegexp *regexp.Regexp
	var err error
	if b.Mode.Expression != "" {
		if exprRegexp, err = regexp.Compile(b.Mode.Expression); err != nil {
			FallbackErrorf("unable to compile expression %q for writer %q: %v", b.Mode.Expression, b.Name, err)
		}
	}

	var buf []byte
	for {
		pause := b.LoggerImpl.GetPauseChan()
		if pause != nil {
			select {
			case <-pause:
			case <-ctx.Done():
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case event, ok := <-b.Queue:
			if !ok {
				return
			}

			if exprRegexp != nil {
				matched := exprRegexp.Match([]byte(fmt.Sprintf("%s:%d:%s", event.Filename, event.Line, event.Caller))) ||
					exprRegexp.Match([]byte(event.Msg))
				if !matched {
					continue
				}
			}

			buf = EventFormatTextMessage(b.Mode, event, buf[:0])
			_, err := b.OutputWriteCloser.Write(buf)
			if err != nil {
				FallbackErrorf("unable to write log message of %q (%v): %s", b.Name, err, string(buf))
			}
			if len(buf) > 2048 {
				buf = nil // do not waste too much memory
			}
		}
	}
}

func NewEventWriterBase(name string, mode WriterMode) *EventWriterBaseImpl {
	if mode.BufferLen == 0 {
		mode.BufferLen = 1000
	}
	if mode.Level == UNDEFINED {
		mode.Level = INFO
	}
	if mode.StacktraceLevel == UNDEFINED {
		mode.StacktraceLevel = NONE
	}
	b := &EventWriterBaseImpl{
		Name:    name,
		Mode:    &mode,
		Queue:   make(chan *Event, mode.BufferLen),
		stopped: make(chan struct{}),
	}
	return b
}

func eventWriterStartGo(ctx context.Context, w EventWriter) {
	go func() {
		defer close(w.Base().stopped)
		w.Run(ctx)
	}()
}

func eventWriterStopWait(w EventWriter) {
	close(w.Base().Queue)
	select {
	case <-w.Base().stopped:
	case <-time.After(2 * time.Second):
		FallbackErrorf("unable to stop log writer %q in time, skip", w.GetWriterName())
	}
}
