// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

// BaseLogger provides the basic logging functions
type BaseLogger interface {
	Log(skip int, level Level, format string, v ...any)
	GetLevel() Level
}

// LevelLogger provides level-related logging functions
type LevelLogger interface {
	LevelEnabled(level Level) bool

	Trace(format string, v ...any)
	Debug(format string, v ...any)
	Info(format string, v ...any)
	Warn(format string, v ...any)
	Error(format string, v ...any)
	Critical(format string, v ...any)
}

type Logger interface {
	BaseLogger
	LevelLogger
}

type baseToLogger struct {
	base BaseLogger
}

var _ Logger = (*baseToLogger)(nil)

func (s *baseToLogger) Log(skip int, level Level, format string, v ...any) {
	s.base.Log(skip+1, level, format, v...)
}

func (s *baseToLogger) GetLevel() Level {
	return s.base.GetLevel()
}

func (s *baseToLogger) LevelEnabled(level Level) bool {
	return s.base.GetLevel() <= level
}

func (s *baseToLogger) Trace(format string, v ...any) {
	s.base.Log(1, TRACE, format, v...)
}

func (s *baseToLogger) Debug(format string, v ...any) {
	s.base.Log(1, DEBUG, format, v...)
}

func (s *baseToLogger) Info(format string, v ...any) {
	s.base.Log(1, INFO, format, v...)
}

func (s *baseToLogger) Warn(format string, v ...any) {
	s.base.Log(1, WARN, format, v...)
}

func (s *baseToLogger) Error(format string, v ...any) {
	s.base.Log(1, ERROR, format, v...)
}

func (s *baseToLogger) Critical(format string, v ...any) {
	s.base.Log(1, CRITICAL, format, v...)
}

// BaseLoggerToGeneralLogger wraps a BaseLogger (which only has Log() function) to a Logger (which has Info() function)
func BaseLoggerToGeneralLogger(b BaseLogger) Logger {
	l := &baseToLogger{base: b}
	return l
}
