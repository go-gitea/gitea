// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sync/syncmap"
)

var (
	loggers []*Logger
	// NamedLoggers map of named loggers
	NamedLoggers = make(map[string]*Logger)
	// GitLogger logger for git
	GitLogger *Logger
	prefix    string
)

// NewLogger create a logger
func NewLogger(bufLen int64, mode, config string) *Logger {
	logger := newLogger(bufLen)

	isExist := false
	for i, l := range loggers {
		if l.adapter == mode {
			isExist = true
			loggers[i] = logger
		}
	}
	if !isExist {
		loggers = append(loggers, logger)
	}
	if err := logger.SetLogger(mode, config); err != nil {
		Critical(1, "Failed to set logger (%s): %v", mode, err)
		panic(fmt.Errorf("Failed to set logger (%s): %v", mode, err))
	}
	return logger
}

// NewNamedLogger creates a new named logger for a given configuration
func NewNamedLogger(name, mode, config string) error {
	l := newLogger(0)
	if err := l.SetLogger(mode, config); err != nil {
		return err
	}
	NamedLoggers[name] = l
	return nil
}

// DelNamedLogger closes and deletes the named logger
func DelNamedLogger(name string) {
	l, ok := NamedLoggers[name]
	if ok {
		delete(NamedLoggers, name)
		l.Close()
	}
}

// DelLogger removes loggers that are for the given mode
func DelLogger(mode string) error {
	for _, l := range loggers {
		if _, ok := l.outputs.Load(mode); ok {
			return l.DelLogger(mode)
		}
	}

	Trace("Log adapter %s not found, no need to delete", mode)
	return nil
}

// FIXME: These two methods should be destroyed...

// NewGitLogger create a logger for git
// FIXME: use same log level as other loggers.
func NewGitLogger(logPath string) {
	path := path.Dir(logPath)

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		Fatal(0, "Failed to create dir %s: %v", path, err)
	}

	GitLogger = newLogger(0)
	GitLogger.SetLogger("file", fmt.Sprintf(`{"level":"TRACE","filename":"%s","rotate":false}`, logPath))
}

//NewAccessLogger creates an access logger
func NewAccessLogger(logPath string) {
	path := path.Dir(logPath)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		Fatal(0, "Failed to create dir %s: %v", path, err)
	}

	//AccessLogger = newLogger(0)
	//AccessLogger.SetLogger("file", fmt.Sprintf(`{"level":0,"filename":"%s","rotate":false, "flags": -1}`, logPath))
}

// GetLevel returns the minimum logger level
func GetLevel() Level {
	level := NONE
	for _, l := range loggers {
		if l.GetLevel() < level {
			level = l.GetLevel()
		}
	}
	return level
}

//NewAccessLogger creates an access logger
func NewAccessLogger(logPath string) {
	path := path.Dir(logPath)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		Fatal(4, "Failed to create dir %s: %v", path, err)
	}

	AccessLogger = newLogger(0)
	AccessLogger.SetLogger("file", fmt.Sprintf(`{"level":0,"filename":"%s","rotate":false, "flags": -1}`, logPath))
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
func Error(skip int, format string, v ...interface{}) {
	Log(skip+1, ERROR, format, v...)
}

// IsError returns true if at least one logger is ERROR
func IsError() bool {
	return GetLevel() <= ERROR
}

// Critical records critical log
func Critical(skip int, format string, v ...interface{}) {
	Log(skip+1, CRITICAL, format, v...)
}

// IsCritical returns true if at least one logger is CRITICAL
func IsCritical() bool {
	return GetLevel() <= CRITICAL
}

// Fatal records error log and exit process
func Fatal(skip int, format string, v ...interface{}) {
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
	for _, l := range loggers {
		l.Close()
	}
}

// Log a message with defined skip and at logging level
func Log(skip int, level Level, format string, v ...interface{}) {
	if GetLevel() > level {
		return
	}
	caller := "?()"
	pc, filename, line, ok := runtime.Caller(skip + 1)
	if ok {
		// Get caller function name.
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			caller = fn.Name() + "()"
		}
	}
	msg := format
	if len(v) > 0 {
		msg = fmt.Sprintf(format, v...)
	}
	for _, l := range loggers {
		l.sendLog(level, caller, strings.TrimPrefix(filename, prefix), line, msg)
	}
}

// .___        __                 _____
// |   | _____/  |_  ____________/ ____\____    ____  ____
// |   |/    \   __\/ __ \_  __ \   __\\__  \ _/ ___\/ __ \
// |   |   |  \  | \  ___/|  | \/|  |   / __ \\  \__\  ___/
// |___|___|  /__|  \___  >__|   |__|  (____  /\___  >___  >
//          \/          \/                  \/     \/    \/

// LoggerInterface represents behaviors of a logger provider.
type LoggerInterface interface {
	Init(config string) error
	LogEvent(event *Event) error
	Close()
	Flush()
	GetLevel() Level
}

type loggerType func() LoggerInterface

// LoggerAsWriter is a io.Writer shim around the gitea log
type LoggerAsWriter struct {
	ourLoggers []*Logger
	level      Level
}

// NewLoggerAsWriter creates a Writer representation of the logger with setable log level
func NewLoggerAsWriter(level string, ourLoggers ...*Logger) *LoggerAsWriter {
	if len(ourLoggers) == 0 {
		ourLoggers = loggers
	}
	l := &LoggerAsWriter{
		ourLoggers: ourLoggers,
		level:      FromString(level),
	}
	return l
}

// Write implements the io.Writer interface to allow spoofing of macaron
func (l *LoggerAsWriter) Write(p []byte) (int, error) {
	l.Log(string(p))
	return len(p), nil
}

// Log takes a given string and logs it at the set log-level
func (l *LoggerAsWriter) Log(msg string) {
	for _, logger := range l.ourLoggers {
		logger.Log(2, l.level, msg)
	}
}

var adapters = make(map[string]loggerType)

// Register registers given logger provider to adapters.
func Register(name string, log loggerType) {
	if log == nil {
		panic("log: register provider is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("log: register called twice for provider \"" + name + "\"")
	}
	adapters[name] = log
}

// Event represents a logging event
type Event struct {
	level    Level
	msg      string
	caller   string
	filename string
	line     int
	time     time.Time
}

// Logger is default logger in the Gitea application.
// it can contain several providers and log message into all providers.
type Logger struct {
	adapter string
	level   Level
	queue   chan *Event
	outputs syncmap.Map
	quit    chan bool
}

// newLogger initializes and returns a new logger.
func newLogger(buffer int64) *Logger {
	l := &Logger{
		queue: make(chan *Event, buffer),
		quit:  make(chan bool),
		level: NONE,
	}
	go l.StartLogger()
	return l
}

// SetLogger sets new logger instance with given logger adapter and config.
func (l *Logger) SetLogger(adapter string, config string) error {
	if log, ok := adapters[adapter]; ok {
		lg := log()
		if err := lg.Init(config); err != nil {
			return err
		}
		l.outputs.Store(adapter, lg)
		l.adapter = adapter
		if lg.GetLevel() < l.level {
			l.level = lg.GetLevel()
		}
	} else {
		panic("log: unknown adapter \"" + adapter + "\" (forgotten register?)")
	}
	return nil
}

// ResetLevel runs through all the loggers and returns the lowest log Level for this logger
func (l *Logger) ResetLevel() Level {
	l.level = NONE
	l.outputs.Range(func(_ interface{}, value interface{}) bool {
		level := value.(LoggerInterface).GetLevel()
		if level < l.level {
			l.level = level
		}
		return true
	})
	return l.level
}

// GetLevel returns the lowest level for this logger
func (l *Logger) GetLevel() Level {
	return l.level
}

// DelLogger removes a logger adapter instance.
func (l *Logger) DelLogger(adapter string) error {
	if lg, ok := l.outputs.Load(adapter); ok {
		lg.(LoggerInterface).Close()
		l.outputs.Delete(adapter)
		// Reset the Level
		l.ResetLevel()
	} else {
		panic("log: unknown adapter \"" + adapter + "\" (forgotten register?)")
	}
	return nil
}

// Log msg at the provided level with the provided caller defined by skip (0 being the function that calls this function)
func (l *Logger) Log(skip int, level Level, format string, v ...interface{}) error {
	if l.level > level {
		return nil
	}
	caller := "?()"
	pc, filename, line, ok := runtime.Caller(skip + 1)
	if ok {
		// Get caller function name.
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			caller = fn.Name() + "()"
		}
	}
	msg := format
	if len(v) > 0 {
		msg = fmt.Sprintf(format, v...)
	}
	return l.sendLog(level, caller, filename, line, msg)
}

func (l *Logger) sendLog(level Level, caller, filename string, line int, msg string) error {
	if l.level > level {
		return nil
	}
	event := &Event{
		level:    level,
		caller:   caller,
		filename: filename,
		line:     line,
		msg:      msg,
		time:     time.Now(),
	}
	l.queue <- event
	return nil
}

// StartLogger starts logger chan reading.
func (l *Logger) StartLogger() {
	for {
		select {
		case event := <-l.queue:
			l.outputs.Range(func(k, v interface{}) bool {
				if err := v.(LoggerInterface).LogEvent(event); err != nil {
					fmt.Println("ERROR, unable to WriteMsg:", err)
				}
				return true
			})
		case <-l.quit:
			return
		}
	}
}

// Flush flushes all chan data.
func (l *Logger) Flush() {
	l.outputs.Range(func(k, v interface{}) bool {
		v.(LoggerInterface).Flush()
		return true
	})
}

// Close closes logger, flush all chan data and destroy all adapter instances.
func (l *Logger) Close() {
	l.quit <- true
	for {
		if len(l.queue) > 0 {
			event := <-l.queue
			l.outputs.Range(func(k, v interface{}) bool {
				if err := v.(LoggerInterface).LogEvent(event); err != nil {
					fmt.Println("ERROR, unable to WriteMsg:", err)
				}
				return true
			})
		} else {
			break
		}
	}
	l.outputs.Range(func(k, v interface{}) bool {
		v.(LoggerInterface).Flush()
		v.(LoggerInterface).Close()
		return true
	})
}

// Trace records trace log
func (l *Logger) Trace(format string, v ...interface{}) {
	l.Log(1, TRACE, format, v...)
}

// Debug records debug log
func (l *Logger) Debug(format string, v ...interface{}) {
	l.Log(1, DEBUG, format, v...)

}

// Info records information log
func (l *Logger) Info(format string, v ...interface{}) {
	l.Log(1, INFO, format, v...)
}

// Warn records warning log
func (l *Logger) Warn(format string, v ...interface{}) {
	l.Log(1, WARN, format, v...)
}

// Error records error log
func (l *Logger) Error(skip int, format string, v ...interface{}) {
	l.Log(skip+1, ERROR, format, v...)
}

// Critical records critical log
func (l *Logger) Critical(skip int, format string, v ...interface{}) {
	l.Log(skip+1, CRITICAL, format, v...)
}

// Fatal records error log and exit the process
func (l *Logger) Fatal(skip int, format string, v ...interface{}) {
	l.Log(skip+1, FATAL, format, v...)
	l.Close()
	os.Exit(1)
}

func init() {
	_, filename, _, _ := runtime.Caller(0)
	prefix = strings.TrimSuffix(filename, "code.gitea.io/gitea/modules/log/log.go")
}
