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

func ParseLevel(name string) (level Level, ok bool) {
	level, ok = _LEVEL_MAP[strings.ToLower(name)]
	return
}

/*

Pair supports logging of key-value pairs.  Keys beginning with _ are
reserved for the logger, e.g. _time, _level, _msg, and _rlevel. The
Pair APIs are designed to avoid heap allocation and garbage
collection.

*/
type Pairs []Pair
type Pair struct {
	Name  string
	Value interface{}
}

/*

Map allows key-value pairs to be specified using map literals or data
structures. For example:

Errorm(msg, Map{...})

Map incurs heap allocation and garbage collection, so the Pair APIs
should be preferred.

*/
type Map map[string]interface{}

// Logger provides a common interface for logging libraries
type Logger interface {
	/*
		These APIs write all the given pairs in addition to standard logger keys.
	*/
	Logp(level Level, msg string, kv ...Pair)

	Debugp(msg string, kv ...Pair)

	Tracep(msg string, kv ...Pair)

	Requestp(rlevel Level, msg string, kv ...Pair)

	Infop(msg string, kv ...Pair)

	Warnp(msg string, kv ...Pair)

	Errorp(msg string, kv ...Pair)

	Severep(msg string, kv ...Pair)

	Fatalp(msg string, kv ...Pair)

	/*
		These APIs write the fields in the given kv Map in addition to standard logger keys.
	*/
	Logm(level Level, msg string, kv Map)

	Debugm(msg string, kv Map)

	Tracem(msg string, kv Map)

	Requestm(rlevel Level, msg string, kv Map)

	Infom(msg string, kv Map)

	Warnm(msg string, kv Map)

	Errorm(msg string, kv Map)

	Severem(msg string, kv Map)

	Fatalm(msg string, kv Map)

	/*

		These APIs only write _msg, _time, _level, and other logger keys. If
		the msg contains other fields, use the Pair or Map APIs instead.

	*/
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

	Level() Level // Get the current logging level
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
}

func Logp(level Level, msg string, kv ...Pair) {
	if skipLogging(level) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Logp(level, msg, kv...)
}

func Debugp(msg string, kv ...Pair) {
	if skipLogging(DEBUG) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Debugp(msg, kv...)
}

func Tracep(msg string, kv ...Pair) {
	if skipLogging(TRACE) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Tracep(msg, kv...)
}

func Requestp(rlevel Level, msg string, kv ...Pair) {
	if skipLogging(REQUEST) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Requestp(rlevel, msg, kv...)
}

func Infop(msg string, kv ...Pair) {
	if skipLogging(INFO) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Infop(msg, kv...)
}

func Warnp(msg string, kv ...Pair) {
	if skipLogging(WARN) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Warnp(msg, kv...)
}

func Errorp(msg string, kv ...Pair) {
	if skipLogging(ERROR) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Errorp(msg, kv...)
}

func Severep(msg string, kv ...Pair) {
	if skipLogging(SEVERE) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Severep(msg, kv...)
}

func Fatalp(msg string, kv ...Pair) {
	if skipLogging(FATAL) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Fatalp(msg, kv...)
}

func Logm(level Level, msg string, kv Map) {
	if skipLogging(level) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Logm(level, msg, kv)
}

func Debugm(msg string, kv Map) {
	if skipLogging(DEBUG) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Debugm(msg, kv)
}

func Tracem(msg string, kv Map) {
	if skipLogging(TRACE) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Tracem(msg, kv)
}

func Requestm(rlevel Level, msg string, kv Map) {
	if skipLogging(REQUEST) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Requestm(rlevel, msg, kv)
}

func Infom(msg string, kv Map) {
	if skipLogging(INFO) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Infom(msg, kv)
}

func Warnm(msg string, kv Map) {
	if skipLogging(WARN) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Warnm(msg, kv)
}

func Errorm(msg string, kv Map) {
	if skipLogging(ERROR) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Errorm(msg, kv)
}

func Severem(msg string, kv Map) {
	if skipLogging(SEVERE) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Severem(msg, kv)
}

func Fatalm(msg string, kv Map) {
	if skipLogging(FATAL) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Fatalm(msg, kv)
}

func Logf(level Level, fmt string, args ...interface{}) {
	if skipLogging(level) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Logf(level, fmt, args...)
}

func Debugf(fmt string, args ...interface{}) {
	if skipLogging(DEBUG) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Debugf(fmt, args...)
}

func Tracef(fmt string, args ...interface{}) {
	if skipLogging(TRACE) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Tracef(fmt, args...)
}

func Requestf(rlevel Level, fmt string, args ...interface{}) {
	if skipLogging(REQUEST) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Requestf(rlevel, fmt, args...)
}

func Infof(fmt string, args ...interface{}) {
	if skipLogging(INFO) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Infof(fmt, args...)
}

func Warnf(fmt string, args ...interface{}) {
	if skipLogging(WARN) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Warnf(fmt, args...)
}

func Errorf(fmt string, args ...interface{}) {
	if skipLogging(ERROR) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Errorf(fmt, args...)
}

func Severef(fmt string, args ...interface{}) {
	if skipLogging(SEVERE) {
		return
	}
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger.Severef(fmt, args...)
}

func Fatalf(fmt string, args ...interface{}) {
	if skipLogging(FATAL) {
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
	logger = NewLogger(os.Stderr, INFO, TEXTFORMATTER)
	SetLogger(logger)
}
