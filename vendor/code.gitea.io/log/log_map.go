// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"sync"
)

// LoggerMap is sync.Map specialised to return loggers
type LoggerMap struct {
	internal sync.Map
}

// Delete a logger from the map
func (m *LoggerMap) Delete(key string) {
	m.internal.Delete(key)
}

// Load returns the logger for the key
func (m *LoggerMap) Load(key string) (*Logger, bool) {
	i, ok := m.internal.Load(key)
	if !ok {
		return nil, ok
	}
	logger := i.(*Logger)
	return logger, ok
}

// LoadOnly returns the logger for the key or nil
func (m *LoggerMap) LoadOnly(key string) *Logger {
	logger, _ := m.Load(key)
	return logger
}

// LoadOrStore returns the existing logger for the key or stores and returns the value
func (m *LoggerMap) LoadOrStore(key string, logger *Logger) (*Logger, bool) {
	i, ok := m.internal.LoadOrStore(key, logger)
	returnable := i.(*Logger)
	return returnable, ok
}

// Store stores the provided logger at the provided key
func (m *LoggerMap) Store(key string, logger *Logger) {
	m.internal.Store(key, logger)
}

// Range calls the provided function for each key and logger in the map
func (m *LoggerMap) Range(f func(string, *Logger) bool) {
	m.internal.Range(func(iKey interface{}, iValue interface{}) bool {
		key := iKey.(string)
		logger := iValue.(*Logger)
		return f(key, logger)
	})
}
