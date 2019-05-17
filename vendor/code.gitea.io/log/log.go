// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package log provides a more comprehensive logging system than that which go provides by default
//
// There are several methods for configuration including programmatically using the NewNamedLogger
// and NewLogger functions. There is also an init provided method to read a log configuration file
// provided as an argument to program: --log-config-file
//
// This configuration file is a JSON file of the form:
// {
//   "DEFAULT_BUFFER_LEN":1000,
//   "default": {
//	   "bufferLen": 1000,
//     "console": {
//       "provider": "console",
//       "config": { "colorize": true, "flags": "stdflags", "stacktraceLevel": "error" }
//     }
//   }
// }
package log

import (
	"fmt"
	"os"
	"strings"
)

var (
	// DEFAULT is the name of the default logger
	DEFAULT = "default"
	// NamedLoggers map of named loggers
	NamedLoggers     = &LoggerMap{}
	prefix           string
	hasDefaultLogger = false
)

// ErrBadConfig represent a bad configuration error
type ErrBadConfig struct {
	message string
}

func (e *ErrBadConfig) Error() string {
	return fmt.Sprintf("Bad Configuration: %s", e.message)
}

// NewLogger create a logger for the default logger
func NewLogger(bufLen int64, name, provider, config string) *Logger {
	if hasDefaultLogger {
		DelLogger("std")
		hasDefaultLogger = false
	}
	err := NewNamedLogger(DEFAULT, bufLen, name, provider, config)
	if err != nil {
		CriticalWithSkip(1, "Unable to create default logger: %v", err)
		panic(err)
	}
	logger, _ := NamedLoggers.Load(DEFAULT)
	return logger
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
func GetLogger(name string) *Logger {
	logger, ok := NamedLoggers.Load(name)
	if ok {
		return logger
	}
	return NamedLoggers.LoadOnly(DEFAULT)
}

// GetLevel returns the minimum logger level
func GetLevel() Level {
	return NamedLoggers.LoadOnly(DEFAULT).GetLevel()
}

// GetStacktraceLevel returns the minimum logger level
func GetStacktraceLevel() Level {
	return NamedLoggers.LoadOnly(DEFAULT).GetStacktraceLevel()
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

// Close closes all the loggers
func Close() {
	NamedLoggers.Range(func(name string, logger *Logger) bool {
		NamedLoggers.Delete(name)
		logger.Close()
		return true
	})
}

// Log a message with defined skip and at logging level
// A skip of 0 refers to the caller of this command
func Log(skip int, level Level, format string, v ...interface{}) {
	l, ok := NamedLoggers.Load(DEFAULT)
	if ok {
		l.Log(skip+1, level, format, v...)
	}
}

// LoggerAsWriter is a io.Writer shim around the gitea log
type LoggerAsWriter struct {
	ourLoggers []*Logger
	level      Level
}

// NewLoggerAsWriter creates a Writer representation of the logger with setable log level
func NewLoggerAsWriter(level string, ourLoggers ...*Logger) *LoggerAsWriter {
	if len(ourLoggers) == 0 {
		ourLoggers = []*Logger{NamedLoggers.LoadOnly(DEFAULT)}
	}
	l := &LoggerAsWriter{
		ourLoggers: ourLoggers,
		level:      FromString(level),
	}
	return l
}

// Write implements the io.Writer interface to allow spoofing of macaron
func (l *LoggerAsWriter) Write(p []byte) (int, error) {
	for _, logger := range l.ourLoggers {
		// Skip = 3 because this presumes that we have been called by log.Println()
		// If the caller has used log.Output or the like this will be wrong
		logger.Log(3, l.level, string(p))
	}
	return len(p), nil
}

// Log takes a given string and logs it at the set log-level
func (l *LoggerAsWriter) Log(msg string) {
	for _, logger := range l.ourLoggers {
		// Set the skip to reference the call just above this
		logger.Log(1, l.level, msg)
	}
}

func init() {
	configFile := ""
	nextArg := false
	for _, arg := range os.Args {
		if nextArg {
			configFile = arg
			break
		}
		if arg == "--log-config-file" {
			nextArg = true
		} else if strings.HasPrefix(arg, "--log-config-file=") {
			configFile = strings.TrimPrefix(arg, "--log-config-file=")
		}
	}
	if _, err := os.Lstat(configFile); err != nil {
		configFile = ""
	}
	var err error

	if configFile != "" {
		err = ConfigureFromFile(configFile)
		if err == nil {
			return
		}
	}

	// create a default log to write to console
	NewLogger(1000, "std", "console", fmt.Sprintf(`{"flags":%d, "stacktraceLevel":"error"}`,
		FlagsFromString("shortfile,shortfuncname,level,microseconds,date,shortfile")))
	hasDefaultLogger = true
	if err != nil {
		Error("Error reading provided config file: %s Err: %v", configFile, err)
	}
}
