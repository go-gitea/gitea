// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"sync"
	"sync/atomic"
)

const DEFAULT = "default"

type LoggerManager struct {
	mu            sync.Mutex
	loggers       map[string]*LoggerImpl
	defaultLogger atomic.Pointer[LoggerImpl]
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
		logger = NewLoggerWithWriters()
		m.loggers[name] = logger
		if name == DEFAULT {
			m.defaultLogger.Store(logger)
		}
	}

	return logger
}

func (m *LoggerManager) PauseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, logger := range m.loggers {
		logger.Pause()
	}
}

func (m *LoggerManager) ResumeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, logger := range m.loggers {
		logger.Resume()
	}
}

func (m *LoggerManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, logger := range m.loggers {
		logger.Close()
	}
}

func (m *LoggerManager) DumpLoggers() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	dump := map[string]any{}
	for name, logger := range m.loggers {
		m := map[string]any{
			"IsEnabled":    logger.IsEnabled(),
			"EventWriters": logger.DumpWriters(),
		}
		dump[name] = m
	}
	return dump
}

var manager *LoggerManager

func GetManager() *LoggerManager {
	return manager
}

func init() {
	manager = &LoggerManager{
		loggers: map[string]*LoggerImpl{},
	}
}
