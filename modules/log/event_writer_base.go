// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"runtime/pprof"
	"time"
)

// EventWriterBase is the base interface for most event writers
// It provides default implementations for most methods
type EventWriterBase interface {
	Base() *EventWriterBaseImpl
	GetWriterType() string
	GetWriterName() string
	GetLevel() Level

	Run(ctx context.Context)
}

type EventWriterBaseImpl struct {
	writerType string

	Name  string
	Mode  *WriterMode
	Queue chan *EventFormatted

	FormatMessage     EventFormatter // format the Event to a message and write it to output
	OutputWriteCloser io.WriteCloser // it will be closed when the event writer is stopped
	GetPauseChan      func() chan struct{}

	shared  bool
	stopped chan struct{}
}

var _ EventWriterBase = (*EventWriterBaseImpl)(nil)

func (b *EventWriterBaseImpl) Base() *EventWriterBaseImpl {
	return b
}

func (b *EventWriterBaseImpl) GetWriterType() string {
	return b.writerType
}

func (b *EventWriterBaseImpl) GetWriterName() string {
	return b.Name
}

func (b *EventWriterBaseImpl) GetLevel() Level {
	return b.Mode.Level
}

// Run is the default implementation for EventWriter.Run
func (b *EventWriterBaseImpl) Run(ctx context.Context) {
	defer b.OutputWriteCloser.Close()

	var exprRegexp *regexp.Regexp
	if b.Mode.Expression != "" {
		var err error
		if exprRegexp, err = regexp.Compile(b.Mode.Expression); err != nil {
			FallbackErrorf("unable to compile expression %q for writer %q: %v", b.Mode.Expression, b.Name, err)
		}
	}

	handlePaused := func() {
		if pause := b.GetPauseChan(); pause != nil {
			select {
			case <-pause:
			case <-ctx.Done():
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-b.Queue:
			if !ok {
				return
			}

			handlePaused()

			if exprRegexp != nil {
				fileLineCaller := fmt.Sprintf("%s:%d:%s", event.Origin.Filename, event.Origin.Line, event.Origin.Caller)
				matched := exprRegexp.MatchString(fileLineCaller) || exprRegexp.MatchString(event.Origin.MsgSimpleText)
				if !matched {
					continue
				}
			}

			var err error
			switch msg := event.Msg.(type) {
			case string:
				_, err = b.OutputWriteCloser.Write([]byte(msg))
			case []byte:
				_, err = b.OutputWriteCloser.Write(msg)
			case io.WriterTo:
				_, err = msg.WriteTo(b.OutputWriteCloser)
			default:
				_, err = b.OutputWriteCloser.Write([]byte(fmt.Sprint(msg)))
			}
			if err != nil {
				FallbackErrorf("unable to write log message of %q (%v): %v", b.Name, err, event.Msg)
			}
		}
	}
}

func NewEventWriterBase(name, writerType string, mode WriterMode) *EventWriterBaseImpl {
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
		writerType: writerType,

		Name:  name,
		Mode:  &mode,
		Queue: make(chan *EventFormatted, mode.BufferLen),

		GetPauseChan:  GetManager().GetPauseChan, // by default, use the global pause channel
		FormatMessage: EventFormatTextMessage,
	}
	return b
}

// eventWriterStartGo use "go" to start an event worker's Run method
func eventWriterStartGo(ctx context.Context, w EventWriter, shared bool) {
	if w.Base().stopped != nil {
		return // already started
	}
	w.Base().shared = shared
	w.Base().stopped = make(chan struct{})

	ctxDesc := "Logger: EventWriter: " + w.GetWriterName()
	if shared {
		ctxDesc = "Logger: EventWriter (shared): " + w.GetWriterName()
	}
	writerCtx, writerCancel := newProcessTypedContext(ctx, ctxDesc)
	go func() {
		defer writerCancel()
		defer close(w.Base().stopped)
		pprof.SetGoroutineLabels(writerCtx)
		w.Run(writerCtx)
	}()
}

// eventWriterStopWait stops an event writer and waits for it to finish flushing (with a timeout)
func eventWriterStopWait(w EventWriter) {
	close(w.Base().Queue)
	select {
	case <-w.Base().stopped:
	case <-time.After(2 * time.Second):
		FallbackErrorf("unable to stop log writer %q in time, skip", w.GetWriterName())
	}
}
