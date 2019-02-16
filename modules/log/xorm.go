// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/go-xorm/core"
)

// XORMLogBridge a logger bridge from Logger to xorm
type XORMLogBridge struct {
	showSQL bool
	level   core.LogLevel
}

var (
	// XORMLogger the logger for xorm
	XORMLogger *XORMLogBridge
)

// DiscardXORMLogger inits a blank logger for xorm
func DiscardXORMLogger() {
	XORMLogger = &XORMLogBridge{
		showSQL: false,
	}
}

// NewXORMLogger generate logger for xorm FIXME: configable
func NewXORMLogger(bufferlen int64, name, adapter, config string) error {
	err := NewNamedLogger("xorm", bufferlen, name, adapter, config)
	if err != nil {
		return err
	}
	if XORMLogger == nil {
		XORMLogger = &XORMLogBridge{
			showSQL: true,
		}
	}
	return nil
}

// GetGiteaLevel returns the minimum Gitea logger level
func (l *XORMLogBridge) GetGiteaLevel() Level {
	logger := NamedLoggers["xorm"]
	if logger != nil {
		return logger.GetLevel()
	}
	return NONE
}

// Log a message with defined skip and at logging level
func (l *XORMLogBridge) Log(skip int, level Level, format string, v ...interface{}) error {
	if l.GetGiteaLevel() > level {
		return nil
	}
	caller := "?()"
	pc, filename, line, ok := runtime.Caller(skip + 2)
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
	return NamedLoggers["xorm"].SendLog(level, caller, strings.TrimPrefix(filename, prefix), line, msg)
}

// Debug show debug log
func (l *XORMLogBridge) Debug(v ...interface{}) {
	if l.level <= core.LOG_DEBUG {
		l.Log(2, DEBUG, fmt.Sprint(v...))
	}
}

// Debugf show debug log
func (l *XORMLogBridge) Debugf(format string, v ...interface{}) {
	if l.level <= core.LOG_DEBUG {
		l.Log(2, DEBUG, format, v...)
	}
}

// Error show error log
func (l *XORMLogBridge) Error(v ...interface{}) {
	if l.level <= core.LOG_ERR {
		l.Log(2, ERROR, fmt.Sprint(v...))
	}
}

// Errorf show error log
func (l *XORMLogBridge) Errorf(format string, v ...interface{}) {
	if l.level <= core.LOG_ERR {
		l.Log(2, ERROR, format, v...)
	}
}

// Info show information level log
func (l *XORMLogBridge) Info(v ...interface{}) {
	if l.level <= core.LOG_INFO {
		l.Log(2, INFO, fmt.Sprint(v...))
	}
}

// Infof show information level log
func (l *XORMLogBridge) Infof(format string, v ...interface{}) {
	if l.level <= core.LOG_INFO {
		l.Log(2, INFO, format, v...)
	}
}

// Warn show warning log
func (l *XORMLogBridge) Warn(v ...interface{}) {
	if l.level <= core.LOG_WARNING {
		l.Log(2, WARN, fmt.Sprint(v...))
	}
}

// Warnf show warnning log
func (l *XORMLogBridge) Warnf(format string, v ...interface{}) {
	if l.level <= core.LOG_WARNING {
		l.Log(2, WARN, format, v...)
	}
}

// Level get logger level
func (l *XORMLogBridge) Level() core.LogLevel {
	return l.level
}

// SetLevel set logger level
func (l *XORMLogBridge) SetLevel(level core.LogLevel) {
	l.level = level
}

// ShowSQL set if record SQL
func (l *XORMLogBridge) ShowSQL(show ...bool) {
	if len(show) > 0 {
		l.showSQL = show[0]
	} else {
		l.showSQL = true
	}
}

// IsShowSQL if record SQL
func (l *XORMLogBridge) IsShowSQL() bool {
	return l.showSQL
}
