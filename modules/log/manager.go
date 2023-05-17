// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

const DEFAULT = "default"

type LoggerManager struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	mu            sync.Mutex
	writers       map[string]EventWriter
	loggers       map[string]*LoggerImpl
	defaultLogger atomic.Pointer[LoggerImpl]

	pauseMu   sync.RWMutex
	pauseChan chan struct{}
}

func (m *LoggerManager) GetLogger(name string) *LoggerImpl {
	if name == DEFAULT {
		if logger := m.defaultLogger.Load(); logger != nil {
			return logger
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	logger := m.loggers[name]
	if logger == nil {
		logger = NewLoggerWithWriters(m.ctx)
		m.loggers[name] = logger
		if name == DEFAULT {
			m.defaultLogger.Store(logger)
		}
	}

	return logger
}

func (m *LoggerManager) PauseAll() {
	m.pauseMu.Lock()
	m.pauseChan = make(chan struct{})
	m.pauseMu.Unlock()
}

func (m *LoggerManager) ResumeAll() {
	m.pauseMu.Lock()
	close(m.pauseChan)
	m.pauseChan = nil
	m.pauseMu.Unlock()
}

func (m *LoggerManager) GetPauseChan() chan struct{} {
	m.pauseMu.RLock()
	defer m.pauseMu.RUnlock()
	return m.pauseChan
}

func (m *LoggerManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, logger := range m.loggers {
		logger.Close()
	}
	m.loggers = map[string]*LoggerImpl{}

	for _, writer := range m.writers {
		eventWriterStopWait(writer)
	}
	m.writers = map[string]EventWriter{}
}

func (m *LoggerManager) DumpLoggers() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	dump := map[string]any{}
	for name, logger := range m.loggers {
		loggerDump := map[string]any{
			"IsEnabled":    logger.IsEnabled(),
			"EventWriters": logger.DumpWriters(),
		}
		dump[name] = loggerDump
	}
	return dump
}

func (m *LoggerManager) GetSharedWriter(writerName string) EventWriter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writers[writerName]
}

func (m *LoggerManager) NewSharedWriter(writerName, writerType string, mode WriterMode) (writer EventWriter, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.writers[writerName]; ok {
		return nil, fmt.Errorf("log event writer %q has been added before", writerName)
	}

	if writer, err = NewEventWriter(writerName, writerType, mode); err != nil {
		return nil, err
	}

	m.writers[writerName] = writer
	eventWriterStartGo(m.ctx, writer, true)
	return writer, nil
}

var loggerManager = NewManager()

func GetManager() *LoggerManager {
	return loggerManager
}

func NewManager() *LoggerManager {
	m := &LoggerManager{writers: map[string]EventWriter{}, loggers: map[string]*LoggerImpl{}}
	m.ctx, m.ctxCancel = context.WithCancel(context.Background())
	return m
}
