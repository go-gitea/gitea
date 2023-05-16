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

// Log settings
var Log struct {
	RootPath string

	Mode               string
	Level              log.Level
	StacktraceLogLevel log.Level
	BufferLen          int

	EnableSSHLog bool

	AccessLogTemplate string
	RequestIDHeaders  []string
}

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

	if !sec.HasKey("logger.DEFAULT.MODE") {
		sec.Key("logger.DEFAULT.MODE").MustString(",")
	}

	deprecatedSetting(rootCfg, "log", "ACCESS", "log", "logger.ACCESS.MODE", "1.21")
	deprecatedSetting(rootCfg, "log", "ENABLE_ACCESS_LOG", "log", "logger.ACCESS.MODE", "1.21")
	if val := sec.Key("ACCESS").String(); val != "" {
		sec.Key("logger.ACCESS.MODE").MustString(val)
	}
	if sec.HasKey("ENABLE_ACCESS_LOG") && !sec.Key("ENABLE_ACCESS_LOG").MustBool() {
		sec.Key("logger.ACCESS.MODE").SetValue("")
	}

	deprecatedSetting(rootCfg, "log", "ROUTER", "log", "logger.ROUTER.MODE", "1.21")
	deprecatedSetting(rootCfg, "log", "DISABLE_ROUTER_LOG", "log", "logger.ROUTER.MODE", "1.21")
	if val := sec.Key("ROUTER").String(); val != "" {
		sec.Key("logger.ROUTER.MODE").MustString(val)
	}
	if !sec.HasKey("logger.ROUTER.MODE") {
		sec.Key("logger.ROUTER.MODE").MustString(",") // use default logger
	}
	if sec.HasKey("DISABLE_ROUTER_LOG") && sec.Key("DISABLE_ROUTER_LOG").MustBool() {
		sec.Key("logger.ROUTER.MODE").SetValue("")
	}

	deprecatedSetting(rootCfg, "log", "XORM", "log", "logger.XORM.MODE", "1.21")
	deprecatedSetting(rootCfg, "log", "ENABLE_XORM_LOG", "log", "logger.XORM.MODE", "1.21")
	if val := sec.Key("XORM").String(); val != "" {
		sec.Key("logger.XORM.MODE").MustString(val)
	}
	if !sec.HasKey("logger.XORM.MODE") {
		sec.Key("logger.XORM.MODE").MustString(",") // use default logger
	}
	if sec.HasKey("ENABLE_XORM_LOG") && !sec.Key("ENABLE_XORM_LOG").MustBool() {
		sec.Key("logger.XORM.MODE").SetValue("")
	}
}

func LogPrepareFilenameForWriter(modeName, fileName string) string {
	defaultFileName := "gitea.log"
	if modeName != "file" {
		defaultFileName = modeName + ".log"
	}
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

func loadLogModeByName(rootCfg ConfigProvider, modeName string) (writerType string, writerMode log.WriterMode) {
	sec := rootCfg.Section("log." + modeName)

	writerMode = log.WriterMode{}
	writerType = KeyInSectionString(sec, "MODE")
	if writerType == "" {
		writerType = modeName
	}
	writerMode.Level = log.LevelFromString(sec.Key("LEVEL").MustString(Log.Level.String()))
	writerMode.StacktraceLevel = log.LevelFromString(sec.Key("STACKTRACE_LEVEL").MustString(Log.StacktraceLogLevel.String()))
	writerMode.Prefix = sec.Key("PREFIX").MustString("")
	writerMode.Flags = log.FlagsFromString(sec.Key("FLAGS").MustString("stdflags"))
	writerMode.Expression = sec.Key("EXPRESSION").MustString("")

	switch writerType {
	case "console":
		useStderr := sec.Key("STDERR").MustBool(false)
		writerOption := log.WriterConsoleOption{Stderr: useStderr}
		if useStderr {
			writerMode.Colorize = sec.Key("COLORIZE").MustBool(log.CanColorStderr)
		} else {
			writerMode.Colorize = sec.Key("COLORIZE").MustBool(log.CanColorStdout)
		}
		writerMode.WriterOption = writerOption
	case "file":
		fileName := LogPrepareFilenameForWriter(modeName, sec.Key("FILE_NAME").String())
		writerOption := log.WriterFileOption{}
		writerOption.FileName = fileName + filenameSuffix // FIXME: the suffix doesn't seem right, see its related comments
		writerOption.LogRotate = sec.Key("LOG_ROTATE").MustBool(true)
		writerOption.MaxSize = 1 << uint(sec.Key("MAX_SIZE_SHIFT").MustInt(28))
		writerOption.DailyRotate = sec.Key("DAILY_ROTATE").MustBool(true)
		writerOption.MaxDays = sec.Key("MAX_DAYS").MustInt(7)
		writerOption.Compress = sec.Key("COMPRESS").MustBool(true)
		writerOption.CompressionLevel = sec.Key("COMPRESSION_LEVEL").MustInt(-1)
		writerMode.WriterOption = writerOption
	case "conn":
		writerOption := log.WriterConnOption{}
		writerOption.ReconnectOnMsg = sec.Key("RECONNECT_ON_MSG").MustBool()
		writerOption.Reconnect = sec.Key("RECONNECT").MustBool()
		writerOption.Protocol = sec.Key("PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"})
		writerOption.Addr = sec.Key("ADDR").MustString(":7020")
		writerMode.WriterOption = writerOption
	default:
		if !log.HasEventWriter(writerType) {
			panic(fmt.Sprintf("invalid log writer type (mode): %s", writerType))
		}
	}

	return writerType, writerMode
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
	loadLogGlobalFrom(CfgProvider)
	prepareLoggerConfig(CfgProvider)

	initLoggerByName(CfgProvider, log.DEFAULT) // default
	initLoggerByName(CfgProvider, "access")
	initLoggerByName(CfgProvider, "router")
	initLoggerByName(CfgProvider, "xorm")

	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.LoggerToWriter(log.GetLogger(log.DEFAULT).Info))
}

func initLoggerByName(rootCfg ConfigProvider, loggerName string) {
	sec := rootCfg.Section("log")
	keyPrefix := "logger." + strings.ToUpper(loggerName)

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
		writerName := modeName
		writerType, writerMode := loadLogModeByName(rootCfg, modeName)
		if writerMode.BufferLen == 0 {
			writerMode.BufferLen = Log.BufferLen
		}
		eventWriter, err := log.NewEventWriter(writerName, writerType, writerMode)
		if err != nil {
			log.FallbackErrorf("Failed to create event writer for logger %s: %v", loggerName, err)
			continue
		}
		eventWriters = append(eventWriters, eventWriter)
	}

	log.GetManager().GetLogger(loggerName).RemoveAllWriters().AddWriters(eventWriters...)
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
