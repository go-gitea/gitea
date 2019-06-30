// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"

	"xorm.io/core"
)

// XORMLogBridge a logger bridge from Logger to xorm
type XORMLogBridge struct {
	showSQL bool
	logger  *log.Logger
}

// NewXORMLogger inits a log bridge for xorm
func NewXORMLogger(showSQL bool) core.ILogger {
	return &XORMLogBridge{
		showSQL: showSQL,
		logger:  log.GetLogger("xorm"),
	}
}

// Log a message with defined skip and at logging level
func (l *XORMLogBridge) Log(skip int, level log.Level, format string, v ...interface{}) error {
	return l.logger.Log(skip+1, level, format, v...)
}

// Debug show debug log
func (l *XORMLogBridge) Debug(v ...interface{}) {
	_ = l.Log(2, log.DEBUG, fmt.Sprint(v...))
}

// Debugf show debug log
func (l *XORMLogBridge) Debugf(format string, v ...interface{}) {
	_ = l.Log(2, log.DEBUG, format, v...)
}

// Error show error log
func (l *XORMLogBridge) Error(v ...interface{}) {
	_ = l.Log(2, log.ERROR, fmt.Sprint(v...))
}

// Errorf show error log
func (l *XORMLogBridge) Errorf(format string, v ...interface{}) {
	_ = l.Log(2, log.ERROR, format, v...)
}

// Info show information level log
func (l *XORMLogBridge) Info(v ...interface{}) {
	_ = l.Log(2, log.INFO, fmt.Sprint(v...))
}

// Infof show information level log
func (l *XORMLogBridge) Infof(format string, v ...interface{}) {
	_ = l.Log(2, log.INFO, format, v...)
}

// Warn show warning log
func (l *XORMLogBridge) Warn(v ...interface{}) {
	_ = l.Log(2, log.WARN, fmt.Sprint(v...))
}

// Warnf show warnning log
func (l *XORMLogBridge) Warnf(format string, v ...interface{}) {
	_ = l.Log(2, log.WARN, format, v...)
}

// Level get logger level
func (l *XORMLogBridge) Level() core.LogLevel {
	switch l.logger.GetLevel() {
	case log.TRACE, log.DEBUG:
		return core.LOG_DEBUG
	case log.INFO:
		return core.LOG_INFO
	case log.WARN:
		return core.LOG_WARNING
	case log.ERROR, log.CRITICAL:
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
