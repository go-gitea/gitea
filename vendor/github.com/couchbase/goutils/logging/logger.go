//  Copyright (c) 2016 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package logging

import (
	"os"
	"runtime"
	"strings"
	"sync"
)

type Level int

const (
	NONE    = Level(iota) // Disable all logging
	FATAL                 // System is in severe error state and has to abort
	SEVERE                // System is in severe error state and cannot recover reliably
	ERROR                 // System is in error state but can recover and continue reliably
	WARN                  // System approaching error state, or is in a correct but undesirable state
	INFO                  // System-level events and status, in correct states
	REQUEST               // Request-level events, with request-specific rlevel
	TRACE                 // Trace detailed system execution, e.g. function entry / exit
	DEBUG                 // Debug
)

type LogEntryFormatter int

const (
	TEXTFORMATTER = LogEntryFormatter(iota)
	JSONFORMATTER
	KVFORMATTER
	UNIFORMFORMATTER
)

func (level Level) String() string {
	return _LEVEL_NAMES[level]
}

var _LEVEL_NAMES = []string{
	DEBUG:   "DEBUG",
	TRACE:   "TRACE",
	REQUEST: "REQUEST",
	INFO:    "INFO",
	WARN:    "WARN",
	ERROR:   "ERROR",
	SEVERE:  "SEVERE",
	FATAL:   "FATAL",
	NONE:    "NONE",
}

var _LEVEL_MAP = map[string]Level{
	"debug":   DEBUG,
	"trace":   TRACE,
	"request": REQUEST,
	"info":    INFO,
	"warn":    WARN,
	"error":   ERROR,
	"severe":  SEVERE,
	"fatal":   FATAL,
	"none":    NONE,
}

// cache logging enablement to improve runtime performance (reduces from multiple tests to a single test on each call)
var (
	cachedDebug   bool
	cachedTrace   bool
	cachedRequest bool
	cachedInfo    bool
	cachedWarn    bool
	cachedError   bool
	cachedSevere  bool
	cachedFatal   bool
)

// maintain the cached logging state
func cacheLoggingChange() {
	cachedDebug = !skipLogging(DEBUG)
	cachedTrace = !skipLogging(TRACE)
	cachedRequest = !skipLogging(REQUEST)
	cachedInfo = !skipLogging(INFO)
	cachedWarn = !skipLogging(WARN)
	cachedError = !skipLogging(ERROR)
	cachedSevere = !skipLogging(SEVERE)
	cachedFatal = !skipLogging(FATAL)
}

func ParseLevel(name string) (level Level, ok bool) {
	level, ok = _LEVEL_MAP[strings.ToLower(name)]
	return
}

// Logger provides a common interface for logging libraries
type Logger interface {
	// Higher performance
	Loga(level Level, f func() string)
	Debuga(f func() string)
	Tracea(f func() string)
	Requesta(rlevel Level, f func() string)
	Infoa(f func() string)
	Warna(f func() string)
	Errora(f func() string)
	Severea(f func() string)
	Fatala(f func() string)

	// Printf style
	Logf(level Level, fmt string, args ...interface{})
	Debugf(fmt string, args ...interface{})
	Tracef(fmt string, args ...interface{})
	Requestf(rlevel Level, fmt string, args ...interface{})
	Infof(fmt string, args ...interface{})
	Warnf(fmt string, args ...interface{})
	Errorf(fmt string, args ...interface{})
	Severef(fmt string, args ...interface{})
	Fatalf(fmt string, args ...interface{})

	/*
		These APIs control the logging level
	*/
	SetLevel(Level) // Set the logging level
	Level() Level   // Get the current logging level
}

var logger Logger = nil
var curLevel Level = DEBUG // initially set to never skip

var loggerMutex sync.RWMutex

// All the methods below first acquire the mutex (mostly in exclusive mode)
// and only then check if logging at the current level is enabled.
// This introduces a fair bottleneck for those log entries that should be
// skipped (the majority, at INFO or below levels)
// We try to predict here if we should lock the mutex at all by caching
// the current log level: while dynamically changing logger, there might
// be the odd entry skipped as the new level is cached.
// Since we seem to never change the logger, this is not an issue.
func skipLogging(level Level) bool {
	if logger == nil {
		return true
	}
	return level > curLevel
}

func SetLogger(newLogger Logger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger = newLogger
	if logger == nil {
		curLevel = NONE
	} else {
		curLevel = newLogger.Level()
	}
	cacheLoggingChange()
}

// we are using deferred unlocking here throughout as we have to do this
// for the anonymous function variants even though it would be more efficient
// to not do this for the printf style variants
// anonymous function variants

func Loga(level Level, f func() string) {
	if skipLogging(level) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Loga(level, f)
}

func Debuga(f func() string) {
	if !cachedDebug {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Debuga(f)
}

func Tracea(f func() string) {
	if !cachedTrace {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Tracea(f)
}

func Requesta(rlevel Level, f func() string) {
	if !cachedRequest {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Requesta(rlevel, f)
}

func Infoa(f func() string) {
	if !cachedInfo {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Infoa(f)
}

func Warna(f func() string) {
	if !cachedWarn {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Warna(f)
}

func Errora(f func() string) {
	if !cachedError {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Errora(f)
}

func Severea(f func() string) {
	if !cachedSevere {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Severea(f)
}

func Fatala(f func() string) {
	if !cachedFatal {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Fatala(f)
}

// printf-style variants

func Logf(level Level, fmt string, args ...interface{}) {
	if skipLogging(level) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Logf(level, fmt, args...)
}

func Debugf(fmt string, args ...interface{}) {
	if !cachedDebug {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Debugf(fmt, args...)
}

func Tracef(fmt string, args ...interface{}) {
	if !cachedTrace {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Tracef(fmt, args...)
}

func Requestf(rlevel Level, fmt string, args ...interface{}) {
	if !cachedRequest {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Requestf(rlevel, fmt, args...)
}

func Infof(fmt string, args ...interface{}) {
	if !cachedInfo {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Infof(fmt, args...)
}

func Warnf(fmt string, args ...interface{}) {
	if !cachedWarn {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Warnf(fmt, args...)
}

func Errorf(fmt string, args ...interface{}) {
	if !cachedError {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Errorf(fmt, args...)
}

func Severef(fmt string, args ...interface{}) {
	if !cachedSevere {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Severef(fmt, args...)
}

func Fatalf(fmt string, args ...interface{}) {
	if !cachedFatal {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Fatalf(fmt, args...)
}

func SetLevel(level Level) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.SetLevel(level)
	curLevel = level
	cacheLoggingChange()
}

func LogLevel() Level {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	return logger.Level()
}

func Stackf(level Level, fmt string, args ...interface{}) {
	if skipLogging(level) {
		return
	}
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, false)
	s := string(buf[0:n])
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Logf(level, fmt, args...)
	logger.Logf(level, s)
}

func init() {
	logger := NewLogger(os.Stderr, INFO, TEXTFORMATTER)
	SetLogger(logger)
}
