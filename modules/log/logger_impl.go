// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

type LoggerImpl struct {
	LevelLogger

	ctx       context.Context
	ctxCancel context.CancelFunc

	level           atomic.Int32
	stacktraceLevel atomic.Int32

	eventWriterMu sync.RWMutex
	eventWriters  map[string]EventWriter
}

var (
	_ BaseLogger  = (*LoggerImpl)(nil)
	_ LevelLogger = (*LoggerImpl)(nil)
)

// SendLogEvent sends a log event to all writers
func (l *LoggerImpl) SendLogEvent(event *Event) {
	l.eventWriterMu.RLock()
	defer l.eventWriterMu.RUnlock()

	if len(l.eventWriters) == 0 {
		FallbackErrorf("[no logger writer]: %s", event.MsgSimpleText)
		return
	}

	// the writers have their own goroutines, the message arguments (with Stringer) shouldn't be used in other goroutines
	// so the event message must be formatted here
	msgFormat, msgArgs := event.msgFormat, event.msgArgs
	event.msgFormat, event.msgArgs = "(already processed by formatters)", nil

	for _, w := range l.eventWriters {
		if event.Level < w.GetLevel() {
			continue
		}
		formatted := &EventFormatted{
			Origin: event,
			Msg:    w.Base().FormatMessage(w.Base().Mode, event, msgFormat, msgArgs...),
		}
		select {
		case w.Base().Queue <- formatted:
		default:
			bs, _ := json.Marshal(event)
			FallbackErrorf("log writer %q queue is full, event: %v", w.GetWriterName(), string(bs))
		}
	}
}

// syncLevelInternal syncs the level of the logger with the levels of the writers
func (l *LoggerImpl) syncLevelInternal() {
	lowestLevel := NONE
	for _, w := range l.eventWriters {
		if w.GetLevel() < lowestLevel {
			lowestLevel = w.GetLevel()
		}
	}
	l.level.Store(int32(lowestLevel))

	lowestLevel = NONE
	for _, w := range l.eventWriters {
		if w.Base().Mode.StacktraceLevel < lowestLevel {
			lowestLevel = w.GetLevel()
		}
	}
	l.stacktraceLevel.Store(int32(lowestLevel))
}

// removeWriterInternal removes a writer from the logger, and stops it if it's not shared
func (l *LoggerImpl) removeWriterInternal(w EventWriter) {
	if !w.Base().shared {
		eventWriterStopWait(w) // only stop non-shared writers, shared writers are managed by the manager
	}
	delete(l.eventWriters, w.GetWriterName())
}

// AddWriters adds writers to the logger, and starts them. Existing writers will be replaced by new ones.
func (l *LoggerImpl) AddWriters(writer ...EventWriter) {
	l.eventWriterMu.Lock()
	defer l.eventWriterMu.Unlock()
	l.addWritersInternal(writer...)
}

func (l *LoggerImpl) addWritersInternal(writer ...EventWriter) {
	for _, w := range writer {
		if old, ok := l.eventWriters[w.GetWriterName()]; ok {
			l.removeWriterInternal(old)
		}
	}

	for _, w := range writer {
		l.eventWriters[w.GetWriterName()] = w
		eventWriterStartGo(l.ctx, w, false)
	}

	l.syncLevelInternal()
}

// RemoveWriter removes a writer from the logger, and the writer is closed and flushed if it is not shared
func (l *LoggerImpl) RemoveWriter(modeName string) error {
	l.eventWriterMu.Lock()
	defer l.eventWriterMu.Unlock()

	w, ok := l.eventWriters[modeName]
	if !ok {
		return util.ErrNotExist
	}

	l.removeWriterInternal(w)
	l.syncLevelInternal()
	return nil
}

// ReplaceAllWriters replaces all writers from the logger, non-shared writers are closed and flushed
func (l *LoggerImpl) ReplaceAllWriters(writer ...EventWriter) {
	l.eventWriterMu.Lock()
	defer l.eventWriterMu.Unlock()

	for _, w := range l.eventWriters {
		l.removeWriterInternal(w)
	}
	l.eventWriters = map[string]EventWriter{}
	l.addWritersInternal(writer...)
}

// DumpWriters dumps the writers as a JSON map, it's used for debugging and display purposes.
func (l *LoggerImpl) DumpWriters() map[string]any {
	l.eventWriterMu.RLock()
	defer l.eventWriterMu.RUnlock()

	writers := make(map[string]any, len(l.eventWriters))
	for k, w := range l.eventWriters {
		bs, err := json.Marshal(w.Base().Mode)
		if err != nil {
			FallbackErrorf("marshal writer %q to dump failed: %v", k, err)
			continue
		}
		m := map[string]any{}
		_ = json.Unmarshal(bs, &m)
		m["WriterType"] = w.GetWriterType()
		writers[k] = m
	}
	return writers
}

// Close closes the logger, non-shared writers are closed and flushed
func (l *LoggerImpl) Close() {
	l.ReplaceAllWriters()
	l.ctxCancel()
}

// IsEnabled returns true if the logger is enabled: it has a working level and has writers
// Fatal is not considered as enabled, because it's a special case and the process just exits
func (l *LoggerImpl) IsEnabled() bool {
	l.eventWriterMu.RLock()
	defer l.eventWriterMu.RUnlock()
	return l.level.Load() < int32(FATAL) && len(l.eventWriters) > 0
}

// Log prepares the log event, if the level matches, the event will be sent to the writers
func (l *LoggerImpl) Log(skip int, level Level, format string, logArgs ...any) {
	if Level(l.level.Load()) > level {
		return
	}

	event := &Event{
		Time:   time.Now(),
		Level:  level,
		Caller: "?()",
	}

	pc, filename, line, ok := runtime.Caller(skip + 1)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			event.Caller = fn.Name() + "()"
		}
	}
	event.Filename, event.Line = strings.TrimPrefix(filename, projectPackagePrefix), line

	if l.stacktraceLevel.Load() <= int32(level) {
		event.Stacktrace = Stack(skip + 1)
	}

	labels := getGoroutineLabels()
	if labels != nil {
		event.GoroutinePid = labels["pid"]
	}

	// get a simple text message without color
	msgArgs := make([]any, len(logArgs))
	copy(msgArgs, logArgs)

	// handle LogStringer values
	for i, v := range msgArgs {
		if cv, ok := v.(*ColoredValue); ok {
			if s, ok := cv.v.(LogStringer); ok {
				cv.v = logStringFormatter{v: s}
			}
		} else if s, ok := v.(LogStringer); ok {
			msgArgs[i] = logStringFormatter{v: s}
		}
	}

	event.MsgSimpleText = colorSprintf(false, format, msgArgs...)
	event.msgFormat = format
	event.msgArgs = msgArgs
	l.SendLogEvent(event)
}

func (l *LoggerImpl) GetLevel() Level {
	return Level(l.level.Load())
}

func NewLoggerWithWriters(ctx context.Context, name string, writer ...EventWriter) *LoggerImpl {
	l := &LoggerImpl{}
	l.ctx, l.ctxCancel = newProcessTypedContext(ctx, "Logger: "+name)
	l.LevelLogger = BaseLoggerToGeneralLogger(l)
	l.eventWriters = map[string]EventWriter{}
	l.AddWriters(writer...)
	return l
}
