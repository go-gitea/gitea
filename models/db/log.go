// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"fmt"
	"sync/atomic"

	"code.gitea.io/gitea/modules/log"

	xormlog "xorm.io/xorm/log"
)

// XORMLogBridge a logger bridge from Logger to xorm
type XORMLogBridge struct {
	showSQLint *int32
	logger     log.Logger
}

// NewXORMLogger inits a log bridge for xorm
func NewXORMLogger(showSQL bool) xormlog.Logger {
	showSQLint := int32(0)
	if showSQL {
		showSQLint = 1
	}
	return &XORMLogBridge{
		showSQLint: &showSQLint,
		logger:     log.GetLogger("xorm"),
	}
}

const stackLevel = 8

// Log a message with defined skip and at logging level
func (l *XORMLogBridge) Log(skip int, level log.Level, format string, v ...interface{}) error {
	return l.logger.Log(skip+1, level, format, v...)
}

// Debug show debug log
func (l *XORMLogBridge) Debug(v ...interface{}) {
	_ = l.Log(stackLevel, log.DEBUG, fmt.Sprint(v...))
}

// Debugf show debug log
func (l *XORMLogBridge) Debugf(format string, v ...interface{}) {
	_ = l.Log(stackLevel, log.DEBUG, format, v...)
}

// Error show error log
func (l *XORMLogBridge) Error(v ...interface{}) {
	_ = l.Log(stackLevel, log.ERROR, fmt.Sprint(v...))
}

// Errorf show error log
func (l *XORMLogBridge) Errorf(format string, v ...interface{}) {
	_ = l.Log(stackLevel, log.ERROR, format, v...)
}

// Info show information level log
func (l *XORMLogBridge) Info(v ...interface{}) {
	_ = l.Log(stackLevel, log.INFO, fmt.Sprint(v...))
}

// Infof show information level log
func (l *XORMLogBridge) Infof(format string, v ...interface{}) {
	_ = l.Log(stackLevel, log.INFO, format, v...)
}

// Warn show warning log
func (l *XORMLogBridge) Warn(v ...interface{}) {
	_ = l.Log(stackLevel, log.WARN, fmt.Sprint(v...))
}

// Warnf show warnning log
func (l *XORMLogBridge) Warnf(format string, v ...interface{}) {
	_ = l.Log(stackLevel, log.WARN, format, v...)
}

// Level get logger level
func (l *XORMLogBridge) Level() xormlog.LogLevel {
	switch l.logger.GetLevel() {
	case log.TRACE, log.DEBUG:
		return xormlog.LOG_DEBUG
	case log.INFO:
		return xormlog.LOG_INFO
	case log.WARN:
		return xormlog.LOG_WARNING
	case log.ERROR, log.CRITICAL:
		return xormlog.LOG_ERR
	}
	return xormlog.LOG_OFF
}

// SetLevel set the logger level
func (l *XORMLogBridge) SetLevel(lvl xormlog.LogLevel) {
}

// ShowSQL set if record SQL
func (l *XORMLogBridge) ShowSQL(show ...bool) {
	showSQL := int32(1)
	if len(show) > 0 && !show[0] {
		showSQL = 0
	}
	atomic.StoreInt32(l.showSQLint, showSQL)
}

// IsShowSQL if record SQL
func (l *XORMLogBridge) IsShowSQL() bool {
	showSQL := atomic.LoadInt32(l.showSQLint)

	return showSQL == 1
}
