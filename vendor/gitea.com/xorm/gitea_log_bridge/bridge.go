// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bridge

import (
	"strings"

	"code.gitea.io/log"
	"github.com/go-xorm/core"
)

var (
	formats           []string
	defaultFormatSize = 20
)

func genFormat(argsLen int) string {
	return strings.TrimSpace(strings.Repeat("%v ", argsLen))
}

func init() {
	formats = make([]string, defaultFormatSize, defaultFormatSize)
	for i := 0; i < defaultFormatSize; i++ {
		formats[i] = genFormat(i)
	}
}

// GiteaLogBridge a logger bridge from Logger to xorm
type GiteaLogBridge struct {
	showSQL bool
	logger  *log.Logger
}

// NewGiteaLogger inits a log bridge for xorm
func NewGiteaLogger(name string, showSQL bool) core.ILogger {
	return &GiteaLogBridge{
		showSQL: showSQL,
		logger:  log.GetLogger(name),
	}
}

// Log a message with defined skip and at logging level
func (l *GiteaLogBridge) Log(skip int, level log.Level, format string, v ...interface{}) error {
	return l.logger.Log(skip+1, level, format, v...)
}

// Debug show debug log
func (l *GiteaLogBridge) Debug(v ...interface{}) {
	l.Log(2, log.DEBUG, formats[len(v)], v...)
}

// Debugf show debug log
func (l *GiteaLogBridge) Debugf(format string, v ...interface{}) {
	l.Log(2, log.DEBUG, format, v...)
}

// Error show error log
func (l *GiteaLogBridge) Error(v ...interface{}) {
	l.Log(2, log.ERROR, formats[len(v)], v...)
}

// Errorf show error log
func (l *GiteaLogBridge) Errorf(format string, v ...interface{}) {
	l.Log(2, log.ERROR, format, v...)
}

// Info show information level log
func (l *GiteaLogBridge) Info(v ...interface{}) {
	l.Log(2, log.INFO, formats[len(v)], v...)
}

// Infof show information level log
func (l *GiteaLogBridge) Infof(format string, v ...interface{}) {
	l.Log(2, log.INFO, format, v...)
}

// Warn show warning log
func (l *GiteaLogBridge) Warn(v ...interface{}) {
	l.Log(2, log.WARN, formats[len(v)], v...)
}

// Warnf show warnning log
func (l *GiteaLogBridge) Warnf(format string, v ...interface{}) {
	l.Log(2, log.WARN, format, v...)
}

// Level get logger level
func (l *GiteaLogBridge) Level() core.LogLevel {
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
func (l *GiteaLogBridge) SetLevel(lvl core.LogLevel) {
}

// ShowSQL set if record SQL
func (l *GiteaLogBridge) ShowSQL(show ...bool) {
	if len(show) > 0 {
		l.showSQL = show[0]
	} else {
		l.showSQL = true
	}
}

// IsShowSQL if record SQL
func (l *GiteaLogBridge) IsShowSQL() bool {
	return l.showSQL
}
