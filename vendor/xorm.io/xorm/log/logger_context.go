// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"context"
	"time"
)

// LogContext represents a log context
type LogContext struct {
	Ctx         context.Context
	SQL         string        // log content or SQL
	Args        []interface{} // if it's a SQL, it's the arguments
	ExecuteTime time.Duration
	Err         error // SQL executed error
}

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

// LoggerAdapter wraps a Logger interafce as LoggerContext interface
type LoggerAdapter struct {
	logger Logger
}

func NewLoggerAdapter(logger Logger) ContextLogger {
	return &LoggerAdapter{
		logger: logger,
	}
}

func (l *LoggerAdapter) BeforeSQL(ctx LogContext) {}

func (l *LoggerAdapter) AfterSQL(ctx LogContext) {
	if ctx.ExecuteTime > 0 {
		l.logger.Infof("[SQL] %v %v - %v", ctx.SQL, ctx.Args, ctx.ExecuteTime)
	} else {
		l.logger.Infof("[SQL] %v %v", ctx.SQL, ctx.Args)
	}
}

func (l *LoggerAdapter) Debugf(format string, v ...interface{}) {
	l.logger.Debugf(format, v...)
}

func (l *LoggerAdapter) Errorf(format string, v ...interface{}) {
	l.logger.Errorf(format, v...)
}

func (l *LoggerAdapter) Infof(format string, v ...interface{}) {
	l.logger.Infof(format, v...)
}

func (l *LoggerAdapter) Warnf(format string, v ...interface{}) {
	l.logger.Warnf(format, v...)
}

func (l *LoggerAdapter) Level() LogLevel {
	return l.logger.Level()
}

func (l *LoggerAdapter) SetLevel(lv LogLevel) {
	l.logger.SetLevel(lv)
}

func (l *LoggerAdapter) ShowSQL(show ...bool) {
	l.logger.ShowSQL(show...)
}

func (l *LoggerAdapter) IsShowSQL() bool {
	return l.logger.IsShowSQL()
}
