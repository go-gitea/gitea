// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package log provides logging capabilities for Gitea.
// Concepts:
//
// * Logger: a Logger provides logging functions and dispatches log events to all its writers
//
// * EventWriter: written log Event to a destination (eg: file, console)
//   - EventWriterBase: the base struct of a writer, it contains common fields and functions for all writers
//   - WriterType: the type name of a writer, eg: console, file
//   - WriterName: aka Mode Name in document, the name of a writer instance, it's usually defined by the config file.
//     It is called "mode name" because old code use MODE as config key, to keep compatibility, keep this concept.
//
// * WriterMode: the common options for all writers, eg: log level.
//   - WriterConsoleOption and others: the specified options for a writer, eg: file path, remote address.
//
// Call graph:
// -> log.Info()
// -> LoggerImpl.Log()
// -> LoggerImpl.SendLogEvent, then the event goes into writer's goroutines
// -> EventWriter.Run() handles the events
package log

// BaseLogger provides the basic logging functions
type BaseLogger interface {
	Log(skip int, event *Event, format string, v ...any)
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

type LogStringer interface { //nolint:revive
	LogString() string
}
