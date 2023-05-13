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
// -> prepare log event, freeze all Stringer arguments (because the Event might be used in another goroutine)
// -> LoggerImpl.SendLogEvent, then the event goes into writer's goroutines
// -> EventWriter.Run() handles the events
package log
