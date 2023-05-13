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

	pauseMu   sync.RWMutex
	pauseChan chan struct{}
}

var (
	_ BaseLogger  = (*LoggerImpl)(nil)
	_ LevelLogger = (*LoggerImpl)(nil)
)

func (l *LoggerImpl) SendLogEvent(event *Event) {
	l.eventWriterMu.RLock()
	defer l.eventWriterMu.RUnlock()

	if len(l.eventWriters) == 0 {
		FallbackErrorf("[no logger writer]: %s", event.Msg)
		return
	}

	for _, w := range l.eventWriters {
		if event.Level < w.GetLevel() {
			continue
		}
		select {
		case w.Base().Queue <- event:
		default:
			bs, _ := json.Marshal(event)
			FallbackErrorf("log writer %q queue is full, event: %v", w.GetWriterName(), string(bs))
		}
	}
}

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

func (l *LoggerImpl) AddWriters(writer ...EventWriter) {
	l.eventWriterMu.Lock()
	defer l.eventWriterMu.Unlock()

	for _, w := range writer {
		if old, ok := l.eventWriters[w.GetWriterName()]; ok {
			eventWriterStopWait(old)
			delete(l.eventWriters, old.GetWriterName())
		}
	}

	for _, w := range writer {
		l.eventWriters[w.GetWriterName()] = w
		w.Base().LoggerImpl = l
		eventWriterStartGo(l.ctx, w)
	}

	l.syncLevelInternal()
}

func (l *LoggerImpl) RemoveWriter(modeName string) error {
	l.eventWriterMu.Lock()
	defer l.eventWriterMu.Unlock()

	w, ok := l.eventWriters[modeName]
	if !ok {
		return util.ErrNotExist
	}

	eventWriterStopWait(w)
	delete(l.eventWriters, w.GetWriterName())
	l.syncLevelInternal()
	return nil
}

func (l *LoggerImpl) RemoveAllWriters() *LoggerImpl {
	l.eventWriterMu.Lock()
	defer l.eventWriterMu.Unlock()

	for _, w := range l.eventWriters {
		eventWriterStopWait(w)
	}
	l.eventWriters = map[string]EventWriter{}
	l.syncLevelInternal()
	return l
}

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

func (l *LoggerImpl) Pause() {
	l.pauseMu.Lock()
	l.pauseChan = make(chan struct{})
	l.pauseMu.Unlock()
}

func (l *LoggerImpl) Resume() {
	l.pauseMu.Lock()
	close(l.pauseChan)
	l.pauseChan = nil
	l.pauseMu.Unlock()
}

func (l *LoggerImpl) GetPauseChan() chan struct{} {
	l.pauseMu.RLock()
	defer l.pauseMu.RUnlock()
	return l.pauseChan
}

func (l *LoggerImpl) IsEnabled() bool {
	l.eventWriterMu.RLock()
	defer l.eventWriterMu.RUnlock()
	return l.level.Load() < int32(FATAL) && len(l.eventWriters) > 0
}

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
	event.Filename, event.Line = strings.TrimPrefix(filename, prefix), line

	if l.stacktraceLevel.Load() <= int32(level) {
		event.Stacktrace = Stack(skip + 1)
	}

	labels := getGoroutineLabels()
	if labels != nil {
		event.GoroutinePid = labels["pid"]
	}

	msgArgs := make([]any, len(logArgs))
	copy(msgArgs, logArgs)
	for i, v := range logArgs {
		if cv, ok := v.(*ColoredValue); ok {
			msgArgs[i] = cv.v
		}
	}
	msg, frozenArgs := frozenMsgFormat(format, msgArgs...)
	for i, v := range logArgs {
		if cv, ok := v.(*ColoredValue); ok {
			newCV := *cv
			newCV.v = frozenArgs[i]
			msgArgs[i] = &newCV
		} else {
			msgArgs[i] = frozenArgs[i]
		}
	}

	event.Msg, event.MsgFormat, event.MsgFrozenArgs = msg, format, msgArgs

	l.SendLogEvent(event)
}

func (l *LoggerImpl) GetLevel() Level {
	return Level(l.level.Load())
}

func NewLoggerWithWriters(writer ...EventWriter) *LoggerImpl {
	l := &LoggerImpl{}
	l.ctx, l.ctxCancel = context.WithCancel(context.Background())
	l.LevelLogger = BaseLoggerToGeneralLogger(l)
	l.eventWriters = map[string]EventWriter{}
	l.syncLevelInternal()
	l.AddWriters(writer...)
	return l
}
