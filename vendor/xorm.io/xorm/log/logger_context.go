// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"

	"xorm.io/xorm/contexts"
)

// LogContext represents a log context
type LogContext contexts.ContextHook

// SQLLogger represents an interface to log SQL
type SQLLogger interface {
	BeforeSQL(context LogContext) // only invoked when IsShowSQL is true
	AfterSQL(context LogContext)  // only invoked when IsShowSQL is true
}

// ContextLogger represents a logger interface with context
type ContextLogger interface {
	SQLLogger

	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})

	Level() LogLevel
	SetLevel(l LogLevel)

	ShowSQL(show ...bool)
	IsShowSQL() bool
}

var (
	_ ContextLogger = &LoggerAdapter{}
)

// enumerate all the context keys
var (
	SessionIDKey      = "__xorm_session_id"
	SessionKey        = "__xorm_session_key"
	SessionShowSQLKey = "__xorm_show_sql"
)

// LoggerAdapter wraps a Logger interface as LoggerContext interface
type LoggerAdapter struct {
	logger Logger
}

// NewLoggerAdapter creates an adapter for old xorm logger interface
func NewLoggerAdapter(logger Logger) ContextLogger {
	return &LoggerAdapter{
		logger: logger,
	}
}

// BeforeSQL implements ContextLogger
func (l *LoggerAdapter) BeforeSQL(ctx LogContext) {}

// AfterSQL implements ContextLogger
func (l *LoggerAdapter) AfterSQL(ctx LogContext) {
	var sessionPart string
	v := ctx.Ctx.Value(SessionIDKey)
	if key, ok := v.(string); ok {
		sessionPart = fmt.Sprintf(" [%s]", key)
	}
	if ctx.ExecuteTime > 0 {
		l.logger.Infof("[SQL]%s %s %v - %v", sessionPart, ctx.SQL, ctx.Args, ctx.ExecuteTime)
	} else {
		l.logger.Infof("[SQL]%s %s %v", sessionPart, ctx.SQL, ctx.Args)
	}
}

// Debugf implements ContextLogger
func (l *LoggerAdapter) Debugf(format string, v ...interface{}) {
	l.logger.Debugf(format, v...)
}

// Errorf implements ContextLogger
func (l *LoggerAdapter) Errorf(format string, v ...interface{}) {
	l.logger.Errorf(format, v...)
}

// Infof implements ContextLogger
func (l *LoggerAdapter) Infof(format string, v ...interface{}) {
	l.logger.Infof(format, v...)
}

// Warnf implements ContextLogger
func (l *LoggerAdapter) Warnf(format string, v ...interface{}) {
	l.logger.Warnf(format, v...)
}

// Level implements ContextLogger
func (l *LoggerAdapter) Level() LogLevel {
	return l.logger.Level()
}

// SetLevel implements ContextLogger
func (l *LoggerAdapter) SetLevel(lv LogLevel) {
	l.logger.SetLevel(lv)
}

// ShowSQL implements ContextLogger
func (l *LoggerAdapter) ShowSQL(show ...bool) {
	l.logger.ShowSQL(show...)
}

// IsShowSQL implements ContextLogger
func (l *LoggerAdapter) IsShowSQL() bool {
	return l.logger.IsShowSQL()
}
