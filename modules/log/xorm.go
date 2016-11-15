package log

import (
	"fmt"

	"github.com/go-xorm/core"
)

// XORMLogBridge a logger bridge from Logger to xorm
type XORMLogBridge struct {
	logger  *Logger
	showSQL bool
	level   core.LogLevel
}

var (
	// XORMLogger the logger for xorm
	XORMLogger *XORMLogBridge
)

// NewXORMLogger generate logger for xorm FIXME: configable
func NewXORMLogger(bufferlen int64, mode, config string) {
	logger := newLogger(bufferlen)
	logger.SetLogger(mode, config)
	XORMLogger = &XORMLogBridge{
		logger:  logger,
		showSQL: true,
	}
}

// Debug show debug log
func (l *XORMLogBridge) Debug(v ...interface{}) {
	if l.level >= core.LOG_DEBUG {
		msg := fmt.Sprint(v...)
		l.logger.writerMsg(0, DEBUG, "[D]"+msg)
	}
}

// Debugf show debug log
func (l *XORMLogBridge) Debugf(format string, v ...interface{}) {
	if l.level >= core.LOG_DEBUG {
		l.logger.Debug(format, v...)
	}
}

// Error show error log
func (l *XORMLogBridge) Error(v ...interface{}) {
	if l.level >= core.LOG_ERR {
		msg := fmt.Sprint(v...)
		l.logger.writerMsg(0, ERROR, "[E]"+msg)
	}
}

// Errorf show error log
func (l *XORMLogBridge) Errorf(format string, v ...interface{}) {
	if l.level >= core.LOG_ERR {
		l.logger.Error(0, format, v...)
	}
}

// Info show information level log
func (l *XORMLogBridge) Info(v ...interface{}) {
	if l.level >= core.LOG_INFO {
		msg := fmt.Sprint(v...)
		l.logger.writerMsg(0, INFO, "[I]"+msg)
	}
}

// Infof show information level log
func (l *XORMLogBridge) Infof(format string, v ...interface{}) {
	if l.level >= core.LOG_INFO {
		l.logger.Info(format, v...)
	}
}

// Warn show warnning log
func (l *XORMLogBridge) Warn(v ...interface{}) {
	if l.level >= core.LOG_WARNING {
		msg := fmt.Sprint(v...)
		l.logger.writerMsg(0, WARN, "[W] "+msg)
	}
}

// Warnf show warnning log
func (l *XORMLogBridge) Warnf(format string, v ...interface{}) {
	if l.level >= core.LOG_WARNING {
		l.logger.Warn(format, v...)
	}
}

// Level get logger level
func (l *XORMLogBridge) Level() core.LogLevel {
	return l.level
}

// SetLevel set logger level
func (l *XORMLogBridge) SetLevel(level core.LogLevel) {
	l.level = level
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
