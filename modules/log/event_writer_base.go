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

	writerType string

	Name  string
	Mode  *WriterMode
	Queue chan *EventFormatted

	FormatMessage     EventFormatter // format the Event to a message and write it to output
	OutputWriteCloser io.WriteCloser // it will be closed when the event writer is stopped

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

func (b *EventWriterBaseImpl) Run(ctx context.Context) {
	defer b.OutputWriteCloser.Close()

	var exprRegexp *regexp.Regexp
	if b.Mode.Expression != "" {
		var err error
		if exprRegexp, err = regexp.Compile(b.Mode.Expression); err != nil {
			FallbackErrorf("unable to compile expression %q for writer %q: %v", b.Mode.Expression, b.Name, err)
		}
	}

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
				fileLineCaller := fmt.Sprintf("%s:%d:%s", event.Origin.Filename, event.Origin.Line, event.Origin.Caller)
				matched := exprRegexp.Match([]byte(fileLineCaller)) || exprRegexp.Match([]byte(event.Origin.MsgSimpleText))
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

		FormatMessage: EventFormatTextMessage,

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
