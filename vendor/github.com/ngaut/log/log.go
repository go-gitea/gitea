//high level log wrapper, so it can output different log based on level
package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	Ldate         = log.Ldate
	Llongfile     = log.Llongfile
	Lmicroseconds = log.Lmicroseconds
	Lshortfile    = log.Lshortfile
	LstdFlags     = log.LstdFlags
	Ltime         = log.Ltime
)

type (
	LogLevel int
	LogType  int
)

const (
	LOG_FATAL   = LogType(0x1)
	LOG_ERROR   = LogType(0x2)
	LOG_WARNING = LogType(0x4)
	LOG_INFO    = LogType(0x8)
	LOG_DEBUG   = LogType(0x10)
)

const (
	LOG_LEVEL_NONE  = LogLevel(0x0)
	LOG_LEVEL_FATAL = LOG_LEVEL_NONE | LogLevel(LOG_FATAL)
	LOG_LEVEL_ERROR = LOG_LEVEL_FATAL | LogLevel(LOG_ERROR)
	LOG_LEVEL_WARN  = LOG_LEVEL_ERROR | LogLevel(LOG_WARNING)
	LOG_LEVEL_INFO  = LOG_LEVEL_WARN | LogLevel(LOG_INFO)
	LOG_LEVEL_DEBUG = LOG_LEVEL_INFO | LogLevel(LOG_DEBUG)
	LOG_LEVEL_ALL   = LOG_LEVEL_DEBUG
)

const FORMAT_TIME_DAY string = "20060102"
const FORMAT_TIME_HOUR string = "2006010215"

var _log *logger = New()

func init() {
	SetFlags(Ldate | Ltime | Lshortfile)
	SetHighlighting(runtime.GOOS != "windows")
}

func Logger() *log.Logger {
	return _log._log
}

func SetLevel(level LogLevel) {
	_log.SetLevel(level)
}
func GetLogLevel() LogLevel {
	return _log.level
}

func SetOutput(out io.Writer) {
	_log.SetOutput(out)
}

func SetOutputByName(path string) error {
	return _log.SetOutputByName(path)
}

func SetFlags(flags int) {
	_log._log.SetFlags(flags)
}

func Info(v ...interface{}) {
	_log.Info(v...)
}

func Infof(format string, v ...interface{}) {
	_log.Infof(format, v...)
}

func Debug(v ...interface{}) {
	_log.Debug(v...)
}

func Debugf(format string, v ...interface{}) {
	_log.Debugf(format, v...)
}

func Warn(v ...interface{}) {
	_log.Warning(v...)
}

func Warnf(format string, v ...interface{}) {
	_log.Warningf(format, v...)
}

func Warning(v ...interface{}) {
	_log.Warning(v...)
}

func Warningf(format string, v ...interface{}) {
	_log.Warningf(format, v...)
}

func Error(v ...interface{}) {
	_log.Error(v...)
}

func Errorf(format string, v ...interface{}) {
	_log.Errorf(format, v...)
}

func Fatal(v ...interface{}) {
	_log.Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	_log.Fatalf(format, v...)
}

func SetLevelByString(level string) {
	_log.SetLevelByString(level)
}

func SetHighlighting(highlighting bool) {
	_log.SetHighlighting(highlighting)
}

func SetRotateByDay() {
	_log.SetRotateByDay()
}

func SetRotateByHour() {
	_log.SetRotateByHour()
}

type logger struct {
	_log         *log.Logger
	level        LogLevel
	highlighting bool

	dailyRolling bool
	hourRolling  bool

	fileName  string
	logSuffix string
	fd        *os.File

	lock sync.Mutex
}

func (l *logger) SetHighlighting(highlighting bool) {
	l.highlighting = highlighting
}

func (l *logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *logger) SetLevelByString(level string) {
	l.level = StringToLogLevel(level)
}

func (l *logger) SetRotateByDay() {
	l.dailyRolling = true
	l.logSuffix = genDayTime(time.Now())
}

func (l *logger) SetRotateByHour() {
	l.hourRolling = true
	l.logSuffix = genHourTime(time.Now())
}

func (l *logger) rotate() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	var suffix string
	if l.dailyRolling {
		suffix = genDayTime(time.Now())
	} else if l.hourRolling {
		suffix = genHourTime(time.Now())
	} else {
		return nil
	}

	// Notice: if suffix is not equal to l.LogSuffix, then rotate
	if suffix != l.logSuffix {
		err := l.doRotate(suffix)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *logger) doRotate(suffix string) error {
	// Notice: Not check error, is this ok?
	l.fd.Close()

	lastFileName := l.fileName + "." + l.logSuffix
	err := os.Rename(l.fileName, lastFileName)
	if err != nil {
		return err
	}

	err = l.SetOutputByName(l.fileName)
	if err != nil {
		return err
	}

	l.logSuffix = suffix

	return nil
}

func (l *logger) SetOutput(out io.Writer) {
	l._log = log.New(out, l._log.Prefix(), l._log.Flags())
}

func (l *logger) SetOutputByName(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	l.SetOutput(f)

	l.fileName = path
	l.fd = f

	return err
}

func (l *logger) log(t LogType, v ...interface{}) {
	if l.level|LogLevel(t) != l.level {
		return
	}

	err := l.rotate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		return
	}

	v1 := make([]interface{}, len(v)+2)
	logStr, logColor := LogTypeToString(t)
	if l.highlighting {
		v1[0] = "\033" + logColor + "m[" + logStr + "]"
		copy(v1[1:], v)
		v1[len(v)+1] = "\033[0m"
	} else {
		v1[0] = "[" + logStr + "]"
		copy(v1[1:], v)
		v1[len(v)+1] = ""
	}

	s := fmt.Sprintln(v1...)
	l._log.Output(4, s)
}

func (l *logger) logf(t LogType, format string, v ...interface{}) {
	if l.level|LogLevel(t) != l.level {
		return
	}

	err := l.rotate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		return
	}

	logStr, logColor := LogTypeToString(t)
	var s string
	if l.highlighting {
		s = "\033" + logColor + "m[" + logStr + "] " + fmt.Sprintf(format, v...) + "\033[0m"
	} else {
		s = "[" + logStr + "] " + fmt.Sprintf(format, v...)
	}
	l._log.Output(4, s)
}

func (l *logger) Fatal(v ...interface{}) {
	l.log(LOG_FATAL, v...)
	os.Exit(-1)
}

func (l *logger) Fatalf(format string, v ...interface{}) {
	l.logf(LOG_FATAL, format, v...)
	os.Exit(-1)
}

func (l *logger) Error(v ...interface{}) {
	l.log(LOG_ERROR, v...)
}

func (l *logger) Errorf(format string, v ...interface{}) {
	l.logf(LOG_ERROR, format, v...)
}

func (l *logger) Warning(v ...interface{}) {
	l.log(LOG_WARNING, v...)
}

func (l *logger) Warningf(format string, v ...interface{}) {
	l.logf(LOG_WARNING, format, v...)
}

func (l *logger) Debug(v ...interface{}) {
	l.log(LOG_DEBUG, v...)
}

func (l *logger) Debugf(format string, v ...interface{}) {
	l.logf(LOG_DEBUG, format, v...)
}

func (l *logger) Info(v ...interface{}) {
	l.log(LOG_INFO, v...)
}

func (l *logger) Infof(format string, v ...interface{}) {
	l.logf(LOG_INFO, format, v...)
}

func StringToLogLevel(level string) LogLevel {
	switch level {
	case "fatal":
		return LOG_LEVEL_FATAL
	case "error":
		return LOG_LEVEL_ERROR
	case "warn":
		return LOG_LEVEL_WARN
	case "warning":
		return LOG_LEVEL_WARN
	case "debug":
		return LOG_LEVEL_DEBUG
	case "info":
		return LOG_LEVEL_INFO
	}
	return LOG_LEVEL_ALL
}

func LogTypeToString(t LogType) (string, string) {
	switch t {
	case LOG_FATAL:
		return "fatal", "[0;31"
	case LOG_ERROR:
		return "error", "[0;31"
	case LOG_WARNING:
		return "warning", "[0;33"
	case LOG_DEBUG:
		return "debug", "[0;36"
	case LOG_INFO:
		return "info", "[0;37"
	}
	return "unknown", "[0;37"
}

func genDayTime(t time.Time) string {
	return t.Format(FORMAT_TIME_DAY)
}

func genHourTime(t time.Time) string {
	return t.Format(FORMAT_TIME_HOUR)
}

func New() *logger {
	return Newlogger(os.Stderr, "")
}

func Newlogger(w io.Writer, prefix string) *logger {
	return &logger{_log: log.New(w, prefix, LstdFlags), level: LOG_LEVEL_ALL, highlighting: true}
}
