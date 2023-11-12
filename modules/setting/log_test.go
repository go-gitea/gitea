// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initLoggersByConfig(t *testing.T, config string) (*log.LoggerManager, func()) {
	oldLogConfig := Log
	Log = LogGlobalConfig{}
	defer func() {
		Log = oldLogConfig
	}()

	cfg, err := NewConfigProviderFromData(config)
	assert.NoError(t, err)

	manager := log.NewManager()
	initManagedLoggers(manager, cfg)
	return manager, manager.Close
}

func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "\t")
	return string(b)
}

func TestLogConfigDefault(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, ``)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))
}

func TestLogConfigDisable(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
logger.router.MODE =
logger.xorm.MODE =
`)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))
}

func TestLogConfigLegacyDefault(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
MODE = console
`)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))
}

func TestLogConfigLegacyMode(t *testing.T) {
	tempDir := t.TempDir()

	tempPath := func(file string) string {
		return filepath.Join(tempDir, file)
	}

	manager, managerClose := initLoggersByConfig(t, `
[log]
ROOT_PATH = `+tempDir+`
MODE = file
ROUTER = file
ACCESS = file
`)
	defer managerClose()

	writerDump := `
{
	"file": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Compress": true,
			"CompressionLevel": -1,
			"DailyRotate": true,
			"FileName": "$FILENAME",
			"LogRotate": true,
			"MaxDays": 7,
			"MaxSize": 268435456
		},
		"WriterType": "file"
	}
}
`
	writerDumpAccess := `
{
	"file.access": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "none",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Compress": true,
			"CompressionLevel": -1,
			"DailyRotate": true,
			"FileName": "$FILENAME",
			"LogRotate": true,
			"MaxDays": 7,
			"MaxSize": 268435456
		},
		"WriterType": "file"
	}
}
`
	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDump, "$FILENAME", tempPath("gitea.log")), toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDumpAccess, "$FILENAME", tempPath("access.log")), toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDump, "$FILENAME", tempPath("gitea.log")), toJSON(dump))
}

func TestLogConfigLegacyModeDisable(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
ROUTER = file
ACCESS = file
DISABLE_ROUTER_LOG = true
ENABLE_ACCESS_LOG = false
`)
	defer managerClose()

	dump := manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))
}

func TestLogConfigNewConfig(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
logger.access.MODE = console
logger.xorm.MODE = console, console-1

[log.console]
LEVEL = warn

[log.console-1]
MODE = console
LEVEL = error
STDERR = true
`)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "warn",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	},
	"console-1": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "error",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": true
		},
		"WriterType": "console"
	}
}
`
	writerDumpAccess := `
{
	"console.access": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "none",
		"Level": "warn",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`
	dump := manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, writerDumpAccess, toJSON(dump))
}

func TestLogConfigModeFile(t *testing.T) {
	tempDir := t.TempDir()

	tempPath := func(file string) string {
		return filepath.Join(tempDir, file)
	}

	manager, managerClose := initLoggersByConfig(t, `
[log]
ROOT_PATH = `+tempDir+`
BUFFER_LEN = 10
MODE = file, file1

[log.file1]
MODE = file
LEVEL = error
STACKTRACE_LEVEL = fatal
EXPRESSION = filter
FLAGS = medfile
PREFIX = "[Prefix] "
FILE_NAME = file-xxx.log
LOG_ROTATE = false
MAX_SIZE_SHIFT = 1
DAILY_ROTATE = false
MAX_DAYS = 90
COMPRESS = false
COMPRESSION_LEVEL = 4
`)
	defer managerClose()

	writerDump := `
{
	"file": {
		"BufferLen": 10,
		"Colorize": false,
		"Expression": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Compress": true,
			"CompressionLevel": -1,
			"DailyRotate": true,
			"FileName": "$FILENAME-0",
			"LogRotate": true,
			"MaxDays": 7,
			"MaxSize": 268435456
		},
		"WriterType": "file"
	},
	"file1": {
		"BufferLen": 10,
		"Colorize": false,
		"Expression": "filter",
		"Flags": "medfile",
		"Level": "error",
		"Prefix": "[Prefix] ",
		"StacktraceLevel": "fatal",
		"WriterOption": {
			"Compress": false,
			"CompressionLevel": 4,
			"DailyRotate": false,
			"FileName": "$FILENAME-1",
			"LogRotate": false,
			"MaxDays": 90,
			"MaxSize": 2
		},
		"WriterType": "file"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	expected := writerDump
	expected = strings.ReplaceAll(expected, "$FILENAME-0", tempPath("gitea.log"))
	expected = strings.ReplaceAll(expected, "$FILENAME-1", tempPath("file-xxx.log"))
	require.JSONEq(t, expected, toJSON(dump))
}
