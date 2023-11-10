// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	golog "log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type LogGlobalConfig struct {
	RootPath string

	Mode               string
	Level              log.Level
	StacktraceLogLevel log.Level
	BufferLen          int

	EnableSSHLog bool

	AccessLogTemplate string
	RequestIDHeaders  []string
}

var Log LogGlobalConfig

const accessLogTemplateDefault = `{{.Ctx.RemoteHost}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}" "{{.Ctx.Req.UserAgent}}"`

func loadLogGlobalFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("log")

	Log.Level = log.LevelFromString(sec.Key("LEVEL").MustString(log.INFO.String()))
	Log.StacktraceLogLevel = log.LevelFromString(sec.Key("STACKTRACE_LEVEL").MustString(log.NONE.String()))
	Log.BufferLen = sec.Key("BUFFER_LEN").MustInt(10000)
	Log.Mode = sec.Key("MODE").MustString("console")

	Log.RootPath = sec.Key("ROOT_PATH").MustString(path.Join(AppWorkPath, "log"))
	if !filepath.IsAbs(Log.RootPath) {
		Log.RootPath = filepath.Join(AppWorkPath, Log.RootPath)
	}
	Log.RootPath = util.FilePathJoinAbs(Log.RootPath)

	Log.EnableSSHLog = sec.Key("ENABLE_SSH_LOG").MustBool(false)

	Log.AccessLogTemplate = sec.Key("ACCESS_LOG_TEMPLATE").MustString(accessLogTemplateDefault)
	Log.RequestIDHeaders = sec.Key("REQUEST_ID_HEADERS").Strings(",")
}

func prepareLoggerConfig(rootCfg ConfigProvider) {
	sec := rootCfg.Section("log")

	if !sec.HasKey("logger.default.MODE") {
		sec.Key("logger.default.MODE").MustString(",")
	}

	deprecatedSetting(rootCfg, "log", "ACCESS", "log", "logger.access.MODE", "1.21")
	deprecatedSetting(rootCfg, "log", "ENABLE_ACCESS_LOG", "log", "logger.access.MODE", "1.21")
	if val := sec.Key("ACCESS").String(); val != "" {
		sec.Key("logger.access.MODE").MustString(val)
	}
	if sec.HasKey("ENABLE_ACCESS_LOG") && !sec.Key("ENABLE_ACCESS_LOG").MustBool() {
		sec.Key("logger.access.MODE").SetValue("")
	}

	deprecatedSetting(rootCfg, "log", "ROUTER", "log", "logger.router.MODE", "1.21")
	deprecatedSetting(rootCfg, "log", "DISABLE_ROUTER_LOG", "log", "logger.router.MODE", "1.21")
	if val := sec.Key("ROUTER").String(); val != "" {
		sec.Key("logger.router.MODE").MustString(val)
	}
	if !sec.HasKey("logger.router.MODE") {
		sec.Key("logger.router.MODE").MustString(",") // use default logger
	}
	if sec.HasKey("DISABLE_ROUTER_LOG") && sec.Key("DISABLE_ROUTER_LOG").MustBool() {
		sec.Key("logger.router.MODE").SetValue("")
	}

	deprecatedSetting(rootCfg, "log", "XORM", "log", "logger.xorm.MODE", "1.21")
	deprecatedSetting(rootCfg, "log", "ENABLE_XORM_LOG", "log", "logger.xorm.MODE", "1.21")
	if val := sec.Key("XORM").String(); val != "" {
		sec.Key("logger.xorm.MODE").MustString(val)
	}
	if !sec.HasKey("logger.xorm.MODE") {
		sec.Key("logger.xorm.MODE").MustString(",") // use default logger
	}
	if sec.HasKey("ENABLE_XORM_LOG") && !sec.Key("ENABLE_XORM_LOG").MustBool() {
		sec.Key("logger.xorm.MODE").SetValue("")
	}
}

func LogPrepareFilenameForWriter(fileName, defaultFileName string) string {
	if fileName == "" {
		fileName = defaultFileName
	}
	if !filepath.IsAbs(fileName) {
		fileName = filepath.Join(Log.RootPath, fileName)
	} else {
		fileName = filepath.Clean(fileName)
	}
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		panic(fmt.Sprintf("unable to create directory for log %q: %v", fileName, err.Error()))
	}
	return fileName
}

func loadLogModeByName(rootCfg ConfigProvider, loggerName, modeName string) (writerName, writerType string, writerMode log.WriterMode, err error) {
	sec := rootCfg.Section("log." + modeName)

	writerMode = log.WriterMode{}
	writerType = ConfigSectionKeyString(sec, "MODE")
	if writerType == "" {
		writerType = modeName
	}

	writerName = modeName
	defaultFlags := "stdflags"
	defaultFilaName := "gitea.log"
	if loggerName == "access" {
		// "access" logger is special, by default it doesn't have output flags, so it also needs a new writer name to avoid conflicting with other writers.
		// so "access" logger's writer name is usually "file.access" or "console.access"
		writerName += ".access"
		defaultFlags = "none"
		defaultFilaName = "access.log"
	}

	writerMode.Level = log.LevelFromString(ConfigInheritedKeyString(sec, "LEVEL", Log.Level.String()))
	writerMode.StacktraceLevel = log.LevelFromString(ConfigInheritedKeyString(sec, "STACKTRACE_LEVEL", Log.StacktraceLogLevel.String()))
	writerMode.Prefix = ConfigInheritedKeyString(sec, "PREFIX")
	writerMode.Expression = ConfigInheritedKeyString(sec, "EXPRESSION")
	writerMode.Flags = log.FlagsFromString(ConfigInheritedKeyString(sec, "FLAGS", defaultFlags))

	switch writerType {
	case "console":
		useStderr := ConfigInheritedKey(sec, "STDERR").MustBool(false)
		defaultCanColor := log.CanColorStdout
		if useStderr {
			defaultCanColor = log.CanColorStderr
		}
		writerOption := log.WriterConsoleOption{Stderr: useStderr}
		writerMode.Colorize = ConfigInheritedKey(sec, "COLORIZE").MustBool(defaultCanColor)
		writerMode.WriterOption = writerOption
	case "file":
		fileName := LogPrepareFilenameForWriter(ConfigInheritedKey(sec, "FILE_NAME").String(), defaultFilaName)
		writerOption := log.WriterFileOption{}
		writerOption.FileName = fileName + filenameSuffix // FIXME: the suffix doesn't seem right, see its related comments
		writerOption.LogRotate = ConfigInheritedKey(sec, "LOG_ROTATE").MustBool(true)
		writerOption.MaxSize = 1 << uint(ConfigInheritedKey(sec, "MAX_SIZE_SHIFT").MustInt(28))
		writerOption.DailyRotate = ConfigInheritedKey(sec, "DAILY_ROTATE").MustBool(true)
		writerOption.MaxDays = ConfigInheritedKey(sec, "MAX_DAYS").MustInt(7)
		writerOption.Compress = ConfigInheritedKey(sec, "COMPRESS").MustBool(true)
		writerOption.CompressionLevel = ConfigInheritedKey(sec, "COMPRESSION_LEVEL").MustInt(-1)
		writerMode.WriterOption = writerOption
	case "conn":
		writerOption := log.WriterConnOption{}
		writerOption.ReconnectOnMsg = ConfigInheritedKey(sec, "RECONNECT_ON_MSG").MustBool()
		writerOption.Reconnect = ConfigInheritedKey(sec, "RECONNECT").MustBool()
		writerOption.Protocol = ConfigInheritedKey(sec, "PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"})
		writerOption.Addr = ConfigInheritedKey(sec, "ADDR").MustString(":7020")
		writerMode.WriterOption = writerOption
	default:
		if !log.HasEventWriter(writerType) {
			return "", "", writerMode, fmt.Errorf("invalid log writer type (mode): %s, maybe it needs something like 'MODE=file' in [log.%s] section", writerType, modeName)
		}
	}

	return writerName, writerType, writerMode, nil
}

var filenameSuffix = ""

// RestartLogsWithPIDSuffix restarts the logs with a PID suffix on files
// FIXME: it seems not right, it breaks log rotating or log collectors
func RestartLogsWithPIDSuffix() {
	filenameSuffix = fmt.Sprintf(".%d", os.Getpid())
	initAllLoggers() // when forking, before restarting, rename logger file and re-init all loggers
}

func InitLoggersForTest() {
	initAllLoggers()
}

// initAllLoggers creates all the log services
func initAllLoggers() {
	initManagedLoggers(log.GetManager(), CfgProvider)

	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.LoggerToWriter(log.GetLogger(log.DEFAULT).Info))
}

func initManagedLoggers(manager *log.LoggerManager, cfg ConfigProvider) {
	loadLogGlobalFrom(cfg)
	prepareLoggerConfig(cfg)

	initLoggerByName(manager, cfg, log.DEFAULT) // default
	initLoggerByName(manager, cfg, "access")
	initLoggerByName(manager, cfg, "router")
	initLoggerByName(manager, cfg, "xorm")
}

func initLoggerByName(manager *log.LoggerManager, rootCfg ConfigProvider, loggerName string) {
	sec := rootCfg.Section("log")
	keyPrefix := "logger." + loggerName

	disabled := sec.HasKey(keyPrefix+".MODE") && sec.Key(keyPrefix+".MODE").String() == ""
	if disabled {
		return
	}

	modeVal := sec.Key(keyPrefix + ".MODE").String()
	if modeVal == "," {
		modeVal = Log.Mode
	}

	var eventWriters []log.EventWriter
	modes := strings.Split(modeVal, ",")
	for _, modeName := range modes {
		modeName = strings.TrimSpace(modeName)
		if modeName == "" {
			continue
		}
		writerName, writerType, writerMode, err := loadLogModeByName(rootCfg, loggerName, modeName)
		if err != nil {
			log.FallbackErrorf("Failed to load writer mode %q for logger %s: %v", modeName, loggerName, err)
			continue
		}
		if writerMode.BufferLen == 0 {
			writerMode.BufferLen = Log.BufferLen
		}
		eventWriter := manager.GetSharedWriter(writerName)
		if eventWriter == nil {
			eventWriter, err = manager.NewSharedWriter(writerName, writerType, writerMode)
			if err != nil {
				log.FallbackErrorf("Failed to create event writer for logger %s: %v", loggerName, err)
				continue
			}
		}
		eventWriters = append(eventWriters, eventWriter)
	}

	manager.GetLogger(loggerName).ReplaceAllWriters(eventWriters...)
}

func InitSQLLoggersForCli(level log.Level) {
	log.SetConsoleLogger("xorm", "console", level)
}

func IsAccessLogEnabled() bool {
	return log.IsLoggerEnabled("access")
}

func IsRouteLogEnabled() bool {
	return log.IsLoggerEnabled("router")
}
