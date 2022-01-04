//  Copyright (c) 2016-2019 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

type goLogger struct {
	logger         *log.Logger
	level          Level
	entryFormatter formatter
}

const (
	_LEVEL  = "_level"
	_MSG    = "_msg"
	_TIME   = "_time"
	_RLEVEL = "_rlevel"
)

func NewLogger(out io.Writer, lvl Level, fmtLogging LogEntryFormatter, fmtArgs ...interface{}) *goLogger {
	logger := &goLogger{
		logger: log.New(out, "", 0),
		level:  lvl,
	}
	if fmtLogging == JSONFORMATTER {
		logger.entryFormatter = &jsonFormatter{}
	} else if fmtLogging == KVFORMATTER {
		logger.entryFormatter = &keyvalueFormatter{}
	} else if fmtLogging == UNIFORMFORMATTER {
		logger.entryFormatter = &uniformFormatter{
			callback: fmtArgs[0].(ComponentCallback),
		}
	} else {
		logger.entryFormatter = &textFormatter{}
	}
	return logger
}

// anonymous function variants

func (gl *goLogger) Loga(level Level, f func() string) {
	if gl.logger == nil {
		return
	}
	if level <= gl.level {
		gl.log(level, NONE, f())
	}
}
func (gl *goLogger) Debuga(f func() string) {
	gl.Loga(DEBUG, f)
}

func (gl *goLogger) Tracea(f func() string) {
	gl.Loga(TRACE, f)
}

func (gl *goLogger) Requesta(rlevel Level, f func() string) {
	if gl.logger == nil {
		return
	}
	if REQUEST <= gl.level {
		gl.log(REQUEST, rlevel, f())
	}
}

func (gl *goLogger) Infoa(f func() string) {
	gl.Loga(INFO, f)
}

func (gl *goLogger) Warna(f func() string) {
	gl.Loga(WARN, f)
}

func (gl *goLogger) Errora(f func() string) {
	gl.Loga(ERROR, f)
}

func (gl *goLogger) Severea(f func() string) {
	gl.Loga(SEVERE, f)
}

func (gl *goLogger) Fatala(f func() string) {
	gl.Loga(FATAL, f)
}

// printf-style variants

func (gl *goLogger) Logf(level Level, format string, args ...interface{}) {
	if gl.logger == nil {
		return
	}
	if level <= gl.level {
		gl.log(level, NONE, fmt.Sprintf(format, args...))
	}
}

func (gl *goLogger) Debugf(format string, args ...interface{}) {
	gl.Logf(DEBUG, format, args...)
}

func (gl *goLogger) Tracef(format string, args ...interface{}) {
	gl.Logf(TRACE, format, args...)
}

func (gl *goLogger) Requestf(rlevel Level, format string, args ...interface{}) {
	if gl.logger == nil {
		return
	}
	if REQUEST <= gl.level {
		gl.log(REQUEST, rlevel, fmt.Sprintf(format, args...))
	}
}

func (gl *goLogger) Infof(format string, args ...interface{}) {
	gl.Logf(INFO, format, args...)
}

func (gl *goLogger) Warnf(format string, args ...interface{}) {
	gl.Logf(WARN, format, args...)
}

func (gl *goLogger) Errorf(format string, args ...interface{}) {
	gl.Logf(ERROR, format, args...)
}

func (gl *goLogger) Severef(format string, args ...interface{}) {
	gl.Logf(SEVERE, format, args...)
}

func (gl *goLogger) Fatalf(format string, args ...interface{}) {
	gl.Logf(FATAL, format, args...)
}

func (gl *goLogger) Level() Level {
	return gl.level
}

func (gl *goLogger) SetLevel(level Level) {
	gl.level = level
}

func (gl *goLogger) log(level Level, rlevel Level, msg string) {
	tm := time.Now().Format("2006-01-02T15:04:05.000-07:00") // time.RFC3339 with milliseconds
	gl.logger.Print(gl.entryFormatter.format(tm, level, rlevel, msg))
}

type formatter interface {
	format(string, Level, Level, string) string
}

type textFormatter struct {
}

// ex. 2016-02-10T09:15:25.498-08:00 [INFO] This is a message from test in text format

func (*textFormatter) format(tm string, level Level, rlevel Level, msg string) string {
	b := &strings.Builder{}
	appendValue(b, tm)
	if rlevel != NONE {
		fmt.Fprintf(b, "[%s,%s] ", level.String(), rlevel.String())
	} else {
		fmt.Fprintf(b, "[%s] ", level.String())
	}
	appendValue(b, msg)
	b.WriteByte('\n')
	return b.String()
}

func appendValue(b *strings.Builder, value interface{}) {
	if _, ok := value.(string); ok {
		fmt.Fprintf(b, "%s ", value)
	} else {
		fmt.Fprintf(b, "%v ", value)
	}
}

type keyvalueFormatter struct {
}

// ex. _time=2016-02-10T09:15:25.498-08:00 _level=INFO _msg=This is a message from test in key-value format

func (*keyvalueFormatter) format(tm string, level Level, rlevel Level, msg string) string {
	b := &strings.Builder{}
	appendKeyValue(b, _TIME, tm)
	appendKeyValue(b, _LEVEL, level.String())
	if rlevel != NONE {
		appendKeyValue(b, _RLEVEL, rlevel.String())
	}
	appendKeyValue(b, _MSG, msg)
	b.WriteByte('\n')
	return b.String()
}

func appendKeyValue(b *strings.Builder, key, value interface{}) {
	if _, ok := value.(string); ok {
		fmt.Fprintf(b, "%v=%s ", key, value)
	} else {
		fmt.Fprintf(b, "%v=%v ", key, value)
	}
}

type jsonFormatter struct {
}

// ex. {"_level":"INFO","_msg":"This is a message from test in json format","_time":"2016-02-10T09:12:59.518-08:00"}

func (*jsonFormatter) format(tm string, level Level, rlevel Level, msg string) string {
	data := make(map[string]interface{}, 4)
	data[_TIME] = tm
	data[_LEVEL] = level.String()
	if rlevel != NONE {
		data[_RLEVEL] = rlevel.String()
	}
	data[_MSG] = msg
	serialized, _ := json.Marshal(data)
	var b strings.Builder
	b.Write(serialized)
	b.WriteByte('\n')
	return b.String()
}

type ComponentCallback func() string

type uniformFormatter struct {
	callback ComponentCallback
}

// ex. 2019-03-15T11:28:07.652-04:00 DEBU COMPONENT.subcomponent This is a message from test in uniform format

var _LEVEL_UNIFORM = []string{
	DEBUG:   "DEBU",
	TRACE:   "TRAC",
	REQUEST: "REQU",
	INFO:    "INFO",
	WARN:    "WARN",
	ERROR:   "ERRO",
	SEVERE:  "SEVE",
	FATAL:   "FATA",
	NONE:    "NONE",
}

func (level Level) UniformString() string {
	return _LEVEL_UNIFORM[level]
}

func (uf *uniformFormatter) format(tm string, level Level, rlevel Level, msg string) string {
	b := &strings.Builder{}
	appendValue(b, tm)
	component := uf.callback()
	if rlevel != NONE {
		// not really any accommodation for a composite level in the uniform standard; just output as abbr,abbr
		fmt.Fprintf(b, "%s,%s %s ", level.UniformString(), rlevel.UniformString(), component)
	} else {
		fmt.Fprintf(b, "%s %s ", level.UniformString(), component)
	}
	appendValue(b, msg)
	b.WriteByte('\n')
	return b.String()
}
