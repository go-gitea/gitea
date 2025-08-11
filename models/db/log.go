// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"
	"sync/atomic"

	"code.gitea.io/gitea/modules/log"

	xormlog "xorm.io/xorm/log"
)

// XORMLogBridge a logger bridge from Logger to xorm
type XORMLogBridge struct {
	showSQL atomic.Bool
	logger  log.Logger
}

// NewXORMLogger inits a log bridge for xorm
func NewXORMLogger(showSQL bool) xormlog.Logger {
	l := &XORMLogBridge{logger: log.GetLogger("xorm")}
	l.showSQL.Store(showSQL)
	return l
}

const stackLevel = 8

// Log a message with defined skip and at logging level
func (l *XORMLogBridge) Log(skip int, level log.Level, format string, v ...any) {
	l.logger.Log(skip+1, &log.Event{Level: level}, format, v...)
}

// Debug show debug log
func (l *XORMLogBridge) Debug(v ...any) {
	l.Log(stackLevel, log.DEBUG, "%s", fmt.Sprint(v...))
}

// Debugf show debug log
func (l *XORMLogBridge) Debugf(format string, v ...any) {
	l.Log(stackLevel, log.DEBUG, format, v...)
}

// Error show error log
func (l *XORMLogBridge) Error(v ...any) {
	l.Log(stackLevel, log.ERROR, "%s", fmt.Sprint(v...))
}

// Errorf show error log
func (l *XORMLogBridge) Errorf(format string, v ...any) {
	l.Log(stackLevel, log.ERROR, format, v...)
}

// Info show information level log
func (l *XORMLogBridge) Info(v ...any) {
	l.Log(stackLevel, log.INFO, "%s", fmt.Sprint(v...))
}

// Infof show information level log
func (l *XORMLogBridge) Infof(format string, v ...any) {
	l.Log(stackLevel, log.INFO, format, v...)
}

// Warn show warning log
func (l *XORMLogBridge) Warn(v ...any) {
	l.Log(stackLevel, log.WARN, "%s", fmt.Sprint(v...))
}

// Warnf show warnning log
func (l *XORMLogBridge) Warnf(format string, v ...any) {
	l.Log(stackLevel, log.WARN, format, v...)
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
	case log.ERROR:
		return xormlog.LOG_ERR
	case log.NONE:
		return xormlog.LOG_OFF
	}
	return xormlog.LOG_UNKNOWN
}

// SetLevel set the logger level
func (l *XORMLogBridge) SetLevel(lvl xormlog.LogLevel) {
}

// ShowSQL set if record SQL
func (l *XORMLogBridge) ShowSQL(show ...bool) {
	if len(show) == 0 {
		show = []bool{true}
	}
	l.showSQL.Store(show[0])
}

// IsShowSQL if record SQL
func (l *XORMLogBridge) IsShowSQL() bool {
	return l.showSQL.Load()
}
