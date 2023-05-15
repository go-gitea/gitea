// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/process"
)

type loggerMap struct {
	sync.Map
}

func (m *loggerMap) Load(k string) (*MultiChannelledLogger, bool) {
	v, ok := m.Map.Load(k)
	if !ok {
		return nil, false
	}
	l, ok := v.(*MultiChannelledLogger)
	return l, ok
}

func (m *loggerMap) Store(k string, v *MultiChannelledLogger) {
	m.Map.Store(k, v)
}

func (m *loggerMap) Delete(k string) {
	m.Map.Delete(k)
}

var (
	// DEFAULT is the name of the default logger
	DEFAULT = "default"
	// NamedLoggers map of named loggers
	NamedLoggers loggerMap
	prefix       string
)

// NewLogger create a logger for the default logger
func NewLogger(bufLen int64, name, provider, config string) *MultiChannelledLogger {
	err := NewNamedLogger(DEFAULT, bufLen, name, provider, config)
	if err != nil {
		CriticalWithSkip(1, "Unable to create default logger: %v", err)
		panic(err)
	}
	l, _ := NamedLoggers.Load(DEFAULT)
	return l
}

// NewNamedLogger creates a new named logger for a given configuration
func NewNamedLogger(name string, bufLen int64, subname, provider, config string) error {
	logger, ok := NamedLoggers.Load(name)
	if !ok {
		logger = newLogger(name, bufLen)
		NamedLoggers.Store(name, logger)
	}

	return logger.SetLogger(subname, provider, config)
}

// DelNamedLogger closes and deletes the named logger
func DelNamedLogger(name string) {
	l, ok := NamedLoggers.Load(name)
	if ok {
		NamedLoggers.Delete(name)
		l.Close()
	}
}

// DelLogger removes the named sublogger from the default logger
func DelLogger(name string) error {
	logger, _ := NamedLoggers.Load(DEFAULT)
	found, err := logger.DelLogger(name)
	if !found {
		Trace("Log %s not found, no need to delete", name)
	}
	return err
}

// GetLogger returns either a named logger or the default logger
func GetLogger(name string) *MultiChannelledLogger {
	logger, ok := NamedLoggers.Load(name)
	if ok {
		return logger
	}
	logger, _ = NamedLoggers.Load(DEFAULT)
	return logger
}

// GetLevel returns the minimum logger level
func GetLevel() Level {
	l, _ := NamedLoggers.Load(DEFAULT)
	return l.GetLevel()
}

// GetStacktraceLevel returns the minimum logger level
func GetStacktraceLevel() Level {
	l, _ := NamedLoggers.Load(DEFAULT)
	return l.GetStacktraceLevel()
}

// Trace records trace log
func Trace(format string, v ...interface{}) {
	Log(1, TRACE, format, v...)
}

// IsTrace returns true if at least one logger is TRACE
func IsTrace() bool {
	return GetLevel() <= TRACE
}

// Debug records debug log
func Debug(format string, v ...interface{}) {
	Log(1, DEBUG, format, v...)
}

// IsDebug returns true if at least one logger is DEBUG
func IsDebug() bool {
	return GetLevel() <= DEBUG
}

// Info records info log
func Info(format string, v ...interface{}) {
	Log(1, INFO, format, v...)
}

// IsInfo returns true if at least one logger is INFO
func IsInfo() bool {
	return GetLevel() <= INFO
}

// Warn records warning log
func Warn(format string, v ...interface{}) {
	Log(1, WARN, format, v...)
}

// IsWarn returns true if at least one logger is WARN
func IsWarn() bool {
	return GetLevel() <= WARN
}

// Error records error log
func Error(format string, v ...interface{}) {
	Log(1, ERROR, format, v...)
}

// ErrorWithSkip records error log from "skip" calls back from this function
func ErrorWithSkip(skip int, format string, v ...interface{}) {
	Log(skip+1, ERROR, format, v...)
}

// IsError returns true if at least one logger is ERROR
func IsError() bool {
	return GetLevel() <= ERROR
}

// Critical records critical log
func Critical(format string, v ...interface{}) {
	Log(1, CRITICAL, format, v...)
}

// CriticalWithSkip records critical log from "skip" calls back from this function
func CriticalWithSkip(skip int, format string, v ...interface{}) {
	Log(skip+1, CRITICAL, format, v...)
}

// IsCritical returns true if at least one logger is CRITICAL
func IsCritical() bool {
	return GetLevel() <= CRITICAL
}

// Fatal records fatal log and exit process
func Fatal(format string, v ...interface{}) {
	Log(1, FATAL, format, v...)
	Close()
	os.Exit(1)
}

// FatalWithSkip records fatal log from "skip" calls back from this function
func FatalWithSkip(skip int, format string, v ...interface{}) {
	Log(skip+1, FATAL, format, v...)
	Close()
	os.Exit(1)
}

// IsFatal returns true if at least one logger is FATAL
func IsFatal() bool {
	return GetLevel() <= FATAL
}

// Pause pauses all the loggers
func Pause() {
	NamedLoggers.Range(func(key, value interface{}) bool {
		logger := value.(*MultiChannelledLogger)
		logger.Pause()
		logger.Flush()
		return true
	})
}

// Resume resumes all the loggers
func Resume() {
	NamedLoggers.Range(func(key, value interface{}) bool {
		logger := value.(*MultiChannelledLogger)
		logger.Resume()
		return true
	})
}

// ReleaseReopen releases and reopens logging files
func ReleaseReopen() error {
	var accumulatedErr error
	NamedLoggers.Range(func(key, value interface{}) bool {
		logger := value.(*MultiChannelledLogger)
		if err := logger.ReleaseReopen(); err != nil {
			if accumulatedErr == nil {
				accumulatedErr = fmt.Errorf("Error reopening %s: %w", key.(string), err)
			} else {
				accumulatedErr = fmt.Errorf("Error reopening %s: %v & %w", key.(string), err, accumulatedErr)
			}
		}
		return true
	})
	return accumulatedErr
}

// Close closes all the loggers
func Close() {
	l, ok := NamedLoggers.Load(DEFAULT)
	if !ok {
		return
	}
	NamedLoggers.Delete(DEFAULT)
	l.Close()
}

// Log a message with defined skip and at logging level
// A skip of 0 refers to the caller of this command
func Log(skip int, level Level, format string, v ...interface{}) {
	l, ok := NamedLoggers.Load(DEFAULT)
	if ok {
		l.Log(skip+1, level, format, v...) //nolint:errcheck
	}
}

// LoggerAsWriter is a io.Writer shim around the gitea log
type LoggerAsWriter struct {
	ourLoggers []*MultiChannelledLogger
	level      Level
}

// NewLoggerAsWriter creates a Writer representation of the logger with setable log level
func NewLoggerAsWriter(level string, ourLoggers ...*MultiChannelledLogger) *LoggerAsWriter {
	if len(ourLoggers) == 0 {
		l, _ := NamedLoggers.Load(DEFAULT)
		ourLoggers = []*MultiChannelledLogger{l}
	}
	l := &LoggerAsWriter{
		ourLoggers: ourLoggers,
		level:      FromString(level),
	}
	return l
}

// Write implements the io.Writer interface to allow spoofing of chi
func (l *LoggerAsWriter) Write(p []byte) (int, error) {
	for _, logger := range l.ourLoggers {
		// Skip = 3 because this presumes that we have been called by log.Println()
		// If the caller has used log.Output or the like this will be wrong
		logger.Log(3, l.level, string(p)) //nolint:errcheck
	}
	return len(p), nil
}

// Log takes a given string and logs it at the set log-level
func (l *LoggerAsWriter) Log(msg string) {
	for _, logger := range l.ourLoggers {
		// Set the skip to reference the call just above this
		_ = logger.Log(1, l.level, msg)
	}
}

func init() {
	process.Trace = func(start bool, pid process.IDType, description string, parentPID process.IDType, typ string) {
		if start && parentPID != "" {
			Log(1, TRACE, "Start %s: %s (from %s) (%s)", NewColoredValue(pid, FgHiYellow), description, NewColoredValue(parentPID, FgYellow), NewColoredValue(typ, Reset))
		} else if start {
			Log(1, TRACE, "Start %s: %s (%s)", NewColoredValue(pid, FgHiYellow), description, NewColoredValue(typ, Reset))
		} else {
			Log(1, TRACE, "Done %s: %s", NewColoredValue(pid, FgHiYellow), NewColoredValue(description, Reset))
		}
	}
	_, filename, _, _ := runtime.Caller(0)
	prefix = strings.TrimSuffix(filename, "modules/log/log.go")
	if prefix == filename {
		// in case the source code file is moved, we can not trim the suffix, the code above should also be updated.
		panic("unable to detect correct package prefix, please update file: " + filename)
	}
}
