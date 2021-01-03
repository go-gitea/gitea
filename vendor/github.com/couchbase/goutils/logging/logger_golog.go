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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

func (gl *goLogger) Logp(level Level, msg string, kv ...Pair) {
	if gl.logger == nil {
		return
	}
	if level <= gl.level {
		e := newLogEntry(msg, level)
		copyPairs(e, kv)
		gl.log(e)
	}
}

func (gl *goLogger) Debugp(msg string, kv ...Pair) {
	gl.Logp(DEBUG, msg, kv...)
}

func (gl *goLogger) Tracep(msg string, kv ...Pair) {
	gl.Logp(TRACE, msg, kv...)
}

func (gl *goLogger) Requestp(rlevel Level, msg string, kv ...Pair) {
	if gl.logger == nil {
		return
	}
	if REQUEST <= gl.level {
		e := newLogEntry(msg, REQUEST)
		e.Rlevel = rlevel
		copyPairs(e, kv)
		gl.log(e)
	}
}

func (gl *goLogger) Infop(msg string, kv ...Pair) {
	gl.Logp(INFO, msg, kv...)
}

func (gl *goLogger) Warnp(msg string, kv ...Pair) {
	gl.Logp(WARN, msg, kv...)
}

func (gl *goLogger) Errorp(msg string, kv ...Pair) {
	gl.Logp(ERROR, msg, kv...)
}

func (gl *goLogger) Severep(msg string, kv ...Pair) {
	gl.Logp(SEVERE, msg, kv...)
}

func (gl *goLogger) Fatalp(msg string, kv ...Pair) {
	gl.Logp(FATAL, msg, kv...)
}

func (gl *goLogger) Logm(level Level, msg string, kv Map) {
	if gl.logger == nil {
		return
	}
	if level <= gl.level {
		e := newLogEntry(msg, level)
		e.Data = kv
		gl.log(e)
	}
}

func (gl *goLogger) Debugm(msg string, kv Map) {
	gl.Logm(DEBUG, msg, kv)
}

func (gl *goLogger) Tracem(msg string, kv Map) {
	gl.Logm(TRACE, msg, kv)
}

func (gl *goLogger) Requestm(rlevel Level, msg string, kv Map) {
	if gl.logger == nil {
		return
	}
	if REQUEST <= gl.level {
		e := newLogEntry(msg, REQUEST)
		e.Rlevel = rlevel
		e.Data = kv
		gl.log(e)
	}
}

func (gl *goLogger) Infom(msg string, kv Map) {
	gl.Logm(INFO, msg, kv)
}

func (gl *goLogger) Warnm(msg string, kv Map) {
	gl.Logm(WARN, msg, kv)
}

func (gl *goLogger) Errorm(msg string, kv Map) {
	gl.Logm(ERROR, msg, kv)
}

func (gl *goLogger) Severem(msg string, kv Map) {
	gl.Logm(SEVERE, msg, kv)
}

func (gl *goLogger) Fatalm(msg string, kv Map) {
	gl.Logm(FATAL, msg, kv)
}

func (gl *goLogger) Logf(level Level, format string, args ...interface{}) {
	if gl.logger == nil {
		return
	}
	if level <= gl.level {
		e := newLogEntry(fmt.Sprintf(format, args...), level)
		gl.log(e)
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
		e := newLogEntry(fmt.Sprintf(format, args...), REQUEST)
		e.Rlevel = rlevel
		gl.log(e)
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

func (gl *goLogger) log(newEntry *logEntry) {
	s := gl.entryFormatter.format(newEntry)
	gl.logger.Print(s)
}

type logEntry struct {
	Time    string
	Level   Level
	Rlevel  Level
	Message string
	Data    Map
}

func newLogEntry(msg string, level Level) *logEntry {
	return &logEntry{
		Time:    time.Now().Format("2006-01-02T15:04:05.000-07:00"), // time.RFC3339 with milliseconds
		Level:   level,
		Rlevel:  NONE,
		Message: msg,
	}
}

func copyPairs(newEntry *logEntry, pairs []Pair) {
	newEntry.Data = make(Map, len(pairs))
	for _, p := range pairs {
		newEntry.Data[p.Name] = p.Value
	}
}

type formatter interface {
	format(*logEntry) string
}

type textFormatter struct {
}

// ex. 2016-02-10T09:15:25.498-08:00 [INFO] This is a message from test in text format

func (*textFormatter) format(newEntry *logEntry) string {
	b := &bytes.Buffer{}
	appendValue(b, newEntry.Time)
	if newEntry.Rlevel != NONE {
		fmt.Fprintf(b, "[%s,%s] ", newEntry.Level.String(), newEntry.Rlevel.String())
	} else {
		fmt.Fprintf(b, "[%s] ", newEntry.Level.String())
	}
	appendValue(b, newEntry.Message)
	for key, value := range newEntry.Data {
		appendKeyValue(b, key, value)
	}
	b.WriteByte('\n')
	s := bytes.NewBuffer(b.Bytes())
	return s.String()
}

func appendValue(b *bytes.Buffer, value interface{}) {
	if _, ok := value.(string); ok {
		fmt.Fprintf(b, "%s ", value)
	} else {
		fmt.Fprintf(b, "%v ", value)
	}
}

type keyvalueFormatter struct {
}

// ex. _time=2016-02-10T09:15:25.498-08:00 _level=INFO _msg=This is a message from test in key-value format

func (*keyvalueFormatter) format(newEntry *logEntry) string {
	b := &bytes.Buffer{}
	appendKeyValue(b, _TIME, newEntry.Time)
	appendKeyValue(b, _LEVEL, newEntry.Level.String())
	if newEntry.Rlevel != NONE {
		appendKeyValue(b, _RLEVEL, newEntry.Rlevel.String())
	}
	appendKeyValue(b, _MSG, newEntry.Message)
	for key, value := range newEntry.Data {
		appendKeyValue(b, key, value)
	}
	b.WriteByte('\n')
	s := bytes.NewBuffer(b.Bytes())
	return s.String()
}

func appendKeyValue(b *bytes.Buffer, key, value interface{}) {
	if _, ok := value.(string); ok {
		fmt.Fprintf(b, "%v=%s ", key, value)
	} else {
		fmt.Fprintf(b, "%v=%v ", key, value)
	}
}

type jsonFormatter struct {
}

// ex. {"_level":"INFO","_msg":"This is a message from test in json format","_time":"2016-02-10T09:12:59.518-08:00"}

func (*jsonFormatter) format(newEntry *logEntry) string {
	if newEntry.Data == nil {
		newEntry.Data = make(Map, 5)
	}
	newEntry.Data[_TIME] = newEntry.Time
	newEntry.Data[_LEVEL] = newEntry.Level.String()
	if newEntry.Rlevel != NONE {
		newEntry.Data[_RLEVEL] = newEntry.Rlevel.String()
	}
	newEntry.Data[_MSG] = newEntry.Message
	serialized, _ := json.Marshal(newEntry.Data)
	s := bytes.NewBuffer(append(serialized, '\n'))
	return s.String()
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

func (uf *uniformFormatter) format(newEntry *logEntry) string {
	b := &bytes.Buffer{}
	appendValue(b, newEntry.Time)
	component := uf.callback()
	if newEntry.Rlevel != NONE {
		// not really any accommodation for a composite level in the uniform standard; just output as abbr,abbr
		fmt.Fprintf(b, "%s,%s %s ", newEntry.Level.UniformString(), newEntry.Rlevel.UniformString(), component)
	} else {
		fmt.Fprintf(b, "%s %s ", newEntry.Level.UniformString(), component)
	}
	appendValue(b, newEntry.Message)
	for key, value := range newEntry.Data {
		appendKeyValue(b, key, value)
	}
	b.WriteByte('\n')
	s := bytes.NewBuffer(b.Bytes())
	return s.String()
}
