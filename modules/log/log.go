// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	loggers []*Logger
	// GitLogger logger for git
	GitLogger *Logger
)

// NewLogger create a logger
func NewLogger(bufLen int64, mode, config string) {
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
		Fatal(2, "Failed to set logger (%s): %v", mode, err)
	}
}

// DelLogger removes loggers that are for the given mode
func DelLogger(mode string) error {
	for _, l := range loggers {
		if _, ok := l.outputs[mode]; ok {
			return l.DelLogger(mode)
		}
	}
	Trace("Log adapter %s not found, no need to delete", mode)
	return nil
}

// NewGitLogger create a logger for git
// FIXME: use same log level as other loggers.
func NewGitLogger(logPath string) {
	path := path.Dir(logPath)

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		Fatal(4, "Failed to create dir %s: %v", path, err)
	}

	GitLogger = newLogger(0)
	GitLogger.SetLogger("file", fmt.Sprintf(`{"level":0,"filename":"%s","rotate":false}`, logPath))
}

// Trace records trace log
func Trace(format string, v ...interface{}) {
	for _, logger := range loggers {
		logger.Trace(format, v...)
	}
}

// Debug records debug log
func Debug(format string, v ...interface{}) {
	for _, logger := range loggers {
		logger.Debug(format, v...)
	}
}

// Info records info log
func Info(format string, v ...interface{}) {
	for _, logger := range loggers {
		logger.Info(format, v...)
	}
}

// Warn records warning log
func Warn(format string, v ...interface{}) {
	for _, logger := range loggers {
		logger.Warn(format, v...)
	}
}

// Error records error log
func Error(skip int, format string, v ...interface{}) {
	for _, logger := range loggers {
		logger.Error(skip, format, v...)
	}
}

// Critical records critical log
func Critical(skip int, format string, v ...interface{}) {
	for _, logger := range loggers {
		logger.Critical(skip, format, v...)
	}
}

// Fatal records error log and exit process
func Fatal(skip int, format string, v ...interface{}) {
	Error(skip, format, v...)
	for _, l := range loggers {
		l.Close()
	}
	os.Exit(1)
}

// Close closes all the loggers
func Close() {
	for _, l := range loggers {
		l.Close()
	}
}

// .___        __                 _____
// |   | _____/  |_  ____________/ ____\____    ____  ____
// |   |/    \   __\/ __ \_  __ \   __\\__  \ _/ ___\/ __ \
// |   |   |  \  | \  ___/|  | \/|  |   / __ \\  \__\  ___/
// |___|___|  /__|  \___  >__|   |__|  (____  /\___  >___  >
//          \/          \/                  \/     \/    \/

// LogLevel level type for log
//type LogLevel int

// log levels
const (
	TRACE = iota
	DEBUG
	INFO
	WARN
	ERROR
	CRITICAL
	FATAL
)

// LoggerInterface represents behaviors of a logger provider.
type LoggerInterface interface {
	Init(config string) error
	WriteMsg(msg string, skip, level int) error
	Destroy()
	Flush()
}

type loggerType func() LoggerInterface

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

type logMsg struct {
	skip, level int
	msg         string
}

// Logger is default logger in beego application.
// it can contain several providers and log message into all providers.
type Logger struct {
	adapter string
	lock    sync.Mutex
	level   int
	msg     chan *logMsg
	outputs map[string]LoggerInterface
	quit    chan bool
}

// newLogger initializes and returns a new logger.
func newLogger(buffer int64) *Logger {
	l := &Logger{
		msg:     make(chan *logMsg, buffer),
		outputs: make(map[string]LoggerInterface),
		quit:    make(chan bool),
	}
	go l.StartLogger()
	return l
}

// SetLogger sets new logger instance with given logger adapter and config.
func (l *Logger) SetLogger(adapter string, config string) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if log, ok := adapters[adapter]; ok {
		lg := log()
		if err := lg.Init(config); err != nil {
			return err
		}
		l.outputs[adapter] = lg
		l.adapter = adapter
	} else {
		panic("log: unknown adapter \"" + adapter + "\" (forgotten register?)")
	}
	return nil
}

// DelLogger removes a logger adapter instance.
func (l *Logger) DelLogger(adapter string) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if lg, ok := l.outputs[adapter]; ok {
		lg.Destroy()
		delete(l.outputs, adapter)
	} else {
		panic("log: unknown adapter \"" + adapter + "\" (forgotten register?)")
	}
	return nil
}

func (l *Logger) writerMsg(skip, level int, msg string) error {
	if l.level > level {
		return nil
	}
	lm := &logMsg{
		skip:  skip,
		level: level,
	}

	// Only error information needs locate position for debugging.
	if lm.level >= ERROR {
		pc, file, line, ok := runtime.Caller(skip)
		if ok {
			// Get caller function name.
			fn := runtime.FuncForPC(pc)
			var fnName string
			if fn == nil {
				fnName = "?()"
			} else {
				fnName = strings.TrimLeft(filepath.Ext(fn.Name()), ".") + "()"
			}

			fileName := file
			if len(fileName) > 20 {
				fileName = "..." + fileName[len(fileName)-20:]
			}
			lm.msg = fmt.Sprintf("[%s:%d %s] %s", fileName, line, fnName, msg)
		} else {
			lm.msg = msg
		}
	} else {
		lm.msg = msg
	}
	l.msg <- lm
	return nil
}

// StartLogger starts logger chan reading.
func (l *Logger) StartLogger() {
	for {
		select {
		case bm := <-l.msg:
			for _, l := range l.outputs {
				if err := l.WriteMsg(bm.msg, bm.skip, bm.level); err != nil {
					fmt.Println("ERROR, unable to WriteMsg:", err)
				}
			}
		case <-l.quit:
			return
		}
	}
}

// Flush flushes all chan data.
func (l *Logger) Flush() {
	for _, l := range l.outputs {
		l.Flush()
	}
}

// Close closes logger, flush all chan data and destroy all adapter instances.
func (l *Logger) Close() {
	l.quit <- true
	for {
		if len(l.msg) > 0 {
			bm := <-l.msg
			for _, l := range l.outputs {
				if err := l.WriteMsg(bm.msg, bm.skip, bm.level); err != nil {
					fmt.Println("ERROR, unable to WriteMsg:", err)
				}
			}
		} else {
			break
		}
	}
	for _, l := range l.outputs {
		l.Flush()
		l.Destroy()
	}
}

// Trace records trace log
func (l *Logger) Trace(format string, v ...interface{}) {
	msg := fmt.Sprintf("[T] "+format, v...)
	l.writerMsg(0, TRACE, msg)
}

// Debug records debug log
func (l *Logger) Debug(format string, v ...interface{}) {
	msg := fmt.Sprintf("[D] "+format, v...)
	l.writerMsg(0, DEBUG, msg)
}

// Info records information log
func (l *Logger) Info(format string, v ...interface{}) {
	msg := fmt.Sprintf("[I] "+format, v...)
	l.writerMsg(0, INFO, msg)
}

// Warn records warning log
func (l *Logger) Warn(format string, v ...interface{}) {
	msg := fmt.Sprintf("[W] "+format, v...)
	l.writerMsg(0, WARN, msg)
}

// Error records error log
func (l *Logger) Error(skip int, format string, v ...interface{}) {
	msg := fmt.Sprintf("[E] "+format, v...)
	l.writerMsg(skip, ERROR, msg)
}

// Critical records critical log
func (l *Logger) Critical(skip int, format string, v ...interface{}) {
	msg := fmt.Sprintf("[C] "+format, v...)
	l.writerMsg(skip, CRITICAL, msg)
}

// Fatal records error log and exit the process
func (l *Logger) Fatal(skip int, format string, v ...interface{}) {
	msg := fmt.Sprintf("[F] "+format, v...)
	l.writerMsg(skip, FATAL, msg)
	l.Close()
	os.Exit(1)
}
