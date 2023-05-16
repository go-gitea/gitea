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

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "\t")
	return string(b)
}

func TestLogConfigDefault(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, ``)

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "date,levelinitial,longfile,shortfile,shortfuncname,time",
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
	defer managerClose()

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

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "date,levelinitial,longfile,shortfile,shortfuncname,time",
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
	defer managerClose()

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

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "date,levelinitial,longfile,shortfile,shortfuncname,time",
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
	defer managerClose()

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
		"Flags": "date,levelinitial,longfile,shortfile,shortfuncname,time",
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
	require.JSONEq(t, strings.ReplaceAll(writerDump, "$FILENAME", tempPath("access.log")), toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDump, "$FILENAME", tempPath("router.log")), toJSON(dump))
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
logger.xorm.MODE = console, console-1

[log.console]
LEVEL = warn

[log.console-1]
MODE = console
LEVEL = error
STDERR = true
`)

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Flags": "date,levelinitial,longfile,shortfile,shortfuncname,time",
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
		"Flags": "date,levelinitial,longfile,shortfile,shortfuncname,time",
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
	defer managerClose()

	dump := manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))
}
