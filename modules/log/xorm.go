// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"

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

// InitXORMLogger inits a log bridge for xorm
func InitXORMLogger(showSQL bool) {
	XORMLogger = &XORMLogBridge{
		showSQL: showSQL,
	}
}

// GetGiteaLevel returns the minimum Gitea logger level
func (l *XORMLogBridge) GetGiteaLevel() Level {
	return GetLogger("xorm").GetLevel()
}

// Log a message with defined skip and at logging level
func (l *XORMLogBridge) Log(skip int, level Level, format string, v ...interface{}) error {
	return GetLogger("xorm").Log(skip+1, level, format, v...)
}

// Debug show debug log
func (l *XORMLogBridge) Debug(v ...interface{}) {
	l.Log(2, DEBUG, fmt.Sprint(v...))
}

// Debugf show debug log
func (l *XORMLogBridge) Debugf(format string, v ...interface{}) {
	l.Log(2, DEBUG, format, v...)
}

// Error show error log
func (l *XORMLogBridge) Error(v ...interface{}) {
	l.Log(2, ERROR, fmt.Sprint(v...))
}

// Errorf show error log
func (l *XORMLogBridge) Errorf(format string, v ...interface{}) {
	l.Log(2, ERROR, format, v...)
}

// Info show information level log
func (l *XORMLogBridge) Info(v ...interface{}) {
	l.Log(2, INFO, fmt.Sprint(v...))
}

// Infof show information level log
func (l *XORMLogBridge) Infof(format string, v ...interface{}) {
	l.Log(2, INFO, format, v...)
}

// Warn show warning log
func (l *XORMLogBridge) Warn(v ...interface{}) {
	l.Log(2, WARN, fmt.Sprint(v...))
}

// Warnf show warnning log
func (l *XORMLogBridge) Warnf(format string, v ...interface{}) {
	l.Log(2, WARN, format, v...)
}

// Level get logger level
func (l *XORMLogBridge) Level() core.LogLevel {
	switch l.GetGiteaLevel() {
	case TRACE, DEBUG:
		return core.LOG_DEBUG
	case INFO:
		return core.LOG_INFO
	case WARN:
		return core.LOG_WARNING
	case ERROR, CRITICAL:
		return core.LOG_ERR
	}
	return core.LOG_OFF
}

// SetLevel set the logger level
func (l *XORMLogBridge) SetLevel(lvl core.LogLevel) {
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
