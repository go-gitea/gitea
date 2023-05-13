// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"io"
)

type LogStringer interface { //nolint:revive
	LogString() string
}

type PrintfLogger struct {
	Logf func(format string, args ...any)
}

func (p *PrintfLogger) Printf(format string, args ...any) {
	p.Logf(format, args...)
}

type loggerToWriter struct {
	logf func(format string, args ...any)
}

func (p *loggerToWriter) Write(bs []byte) (int, error) {
	p.logf("%s", string(bs))
	return len(bs), nil
}

func LoggerToWriter(logf func(format string, args ...any)) io.Writer {
	return &loggerToWriter{logf: logf}
}
