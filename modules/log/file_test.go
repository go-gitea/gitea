// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileLoggerFails(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestFileLogger")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	//filename := filepath.Join(tmpDir, "test.log")

	fileLogger := NewFileLogger()
	//realFileLogger, ok := fileLogger.(*FileLogger)
	//assert.Equal(t, true, ok)

	// Fail if there is bad json
	err = fileLogger.Init("{")
	assert.Error(t, err)

	// Fail if there is no filename
	err = fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\"}", prefix, level.String(), flags, ""))
	assert.Error(t, err)

	// Fail if the file isn't a filename
	err = fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\"}", prefix, level.String(), flags, tmpDir))
	assert.Error(t, err)

}

func TestFileLogger(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestFileLogger")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	filename := filepath.Join(tmpDir, "test.log")

	fileLogger := NewFileLogger()
	realFileLogger, ok := fileLogger.(*FileLogger)
	assert.Equal(t, true, ok)

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	dateString := date.UTC().Format("2006/01/02 15:04:05")

	event := Event{
		level:    INFO,
		msg:      "TEST MSG",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)

	fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\",\"maxsize\":%d}", prefix, level.String(), flags, filename, len(expected)*2))

	assert.Equal(t, flags, realFileLogger.Flags)
	assert.Equal(t, level, realFileLogger.Level)
	assert.Equal(t, level, fileLogger.GetLevel())

	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = DEBUG
	expected = expected + ""
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = TRACE
	expected = expected + ""
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = WARN
	expected = expected + fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	// Should rotate
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename + fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), 1))
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	assert.Equal(t, expected, string(logData))

	for num := 2; num <= 999; num++ {
		file, err := os.OpenFile(filename+fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), num), os.O_RDONLY|os.O_CREATE, 0666)
		assert.NoError(t, err)
		file.Close()
	}
	err = realFileLogger.DoRotate()
	assert.Error(t, err)

	expected = expected + fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	// Should fail to rotate
	expected = expected + fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	fileLogger.Close()
}
