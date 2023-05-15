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
	"sync"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

var (
	filenameSuffix  = ""
	descriptionLock = sync.RWMutex{}
	logDescriptions = make(map[string]*LogDescription)
)

// Log settings
var Log struct {
	Level              log.Level
	StacktraceLogLevel string
	RootPath           string
	EnableSSHLog       bool
	EnableXORMLog      bool

	DisableRouterLog bool

	EnableAccessLog   bool
	AccessLogTemplate string
	BufferLength      int64
	RequestIDHeaders  []string
}

// GetLogDescriptions returns a race safe set of descriptions
func GetLogDescriptions() map[string]*LogDescription {
	descriptionLock.RLock()
	defer descriptionLock.RUnlock()
	descs := make(map[string]*LogDescription, len(logDescriptions))
	for k, v := range logDescriptions {
		subLogDescriptions := make([]SubLogDescription, len(v.SubLogDescriptions))
		copy(subLogDescriptions, v.SubLogDescriptions)

		descs[k] = &LogDescription{
			Name:               v.Name,
			SubLogDescriptions: subLogDescriptions,
		}
	}
	return descs
}

// AddLogDescription adds a set of descriptions to the complete description
func AddLogDescription(key string, description *LogDescription) {
	descriptionLock.Lock()
	defer descriptionLock.Unlock()
	logDescriptions[key] = description
}

// AddSubLogDescription adds a sub log description
func AddSubLogDescription(key string, subLogDescription SubLogDescription) bool {
	descriptionLock.Lock()
	defer descriptionLock.Unlock()
	desc, ok := logDescriptions[key]
	if !ok {
		return false
	}
	for i, sub := range desc.SubLogDescriptions {
		if sub.Name == subLogDescription.Name {
			desc.SubLogDescriptions[i] = subLogDescription
			return true
		}
	}
	desc.SubLogDescriptions = append(desc.SubLogDescriptions, subLogDescription)
	return true
}

// RemoveSubLogDescription removes a sub log description
func RemoveSubLogDescription(key, name string) bool {
	descriptionLock.Lock()
	defer descriptionLock.Unlock()
	desc, ok := logDescriptions[key]
	if !ok {
		return false
	}
	for i, sub := range desc.SubLogDescriptions {
		if sub.Name == name {
			desc.SubLogDescriptions = append(desc.SubLogDescriptions[:i], desc.SubLogDescriptions[i+1:]...)
			return true
		}
	}
	return false
}

type defaultLogOptions struct {
	levelName      string // LogLevel
	flags          string
	filename       string // path.Join(LogRootPath, "gitea.log")
	bufferLength   int64
	disableConsole bool
}

func newDefaultLogOptions() defaultLogOptions {
	return defaultLogOptions{
		levelName:      Log.Level.String(),
		flags:          "stdflags",
		filename:       filepath.Join(Log.RootPath, "gitea.log"),
		bufferLength:   10000,
		disableConsole: false,
	}
}

// SubLogDescription describes a sublogger
type SubLogDescription struct {
	Name     string
	Provider string
	Config   string
}

// LogDescription describes a named logger
type LogDescription struct {
	Name               string
	SubLogDescriptions []SubLogDescription
}

func getLogLevel(section ConfigSection, key string, defaultValue log.Level) log.Level {
	value := section.Key(key).MustString(defaultValue.String())
	return log.FromString(value)
}

func getStacktraceLogLevel(section ConfigSection, key, defaultValue string) string {
	value := section.Key(key).MustString(defaultValue)
	return log.FromString(value).String()
}

func loadLogFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("log")
	Log.Level = getLogLevel(sec, "LEVEL", log.INFO)
	Log.StacktraceLogLevel = getStacktraceLogLevel(sec, "STACKTRACE_LEVEL", "None")
	Log.RootPath = sec.Key("ROOT_PATH").MustString(path.Join(AppWorkPath, "log"))
	forcePathSeparator(Log.RootPath)
	Log.BufferLength = sec.Key("BUFFER_LEN").MustInt64(10000)

	Log.EnableSSHLog = sec.Key("ENABLE_SSH_LOG").MustBool(false)
	Log.EnableAccessLog = sec.Key("ENABLE_ACCESS_LOG").MustBool(false)
	Log.AccessLogTemplate = sec.Key("ACCESS_LOG_TEMPLATE").MustString(
		`{{.Ctx.RemoteHost}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}" "{{.Ctx.Req.UserAgent}}"`,
	)
	Log.RequestIDHeaders = sec.Key("REQUEST_ID_HEADERS").Strings(",")
	// the `MustString` updates the default value, and `log.ACCESS` is used by `generateNamedLogger("access")` later
	_ = rootCfg.Section("log").Key("ACCESS").MustString("file")

	sec.Key("ROUTER").MustString("console")
	// Allow [log]  DISABLE_ROUTER_LOG to override [server] DISABLE_ROUTER_LOG
	Log.DisableRouterLog = sec.Key("DISABLE_ROUTER_LOG").MustBool(Log.DisableRouterLog)

	Log.EnableXORMLog = rootCfg.Section("log").Key("ENABLE_XORM_LOG").MustBool(true)
}

func generateLogConfig(sec ConfigSection, name string, defaults defaultLogOptions) (mode, jsonConfig, levelName string) {
	level := getLogLevel(sec, "LEVEL", Log.Level)
	levelName = level.String()
	stacktraceLevelName := getStacktraceLogLevel(sec, "STACKTRACE_LEVEL", Log.StacktraceLogLevel)
	stacktraceLevel := log.FromString(stacktraceLevelName)
	mode = name
	keys := sec.Keys()
	logPath := defaults.filename
	flags := log.FlagsFromString(defaults.flags)
	expression := ""
	prefix := ""
	for _, key := range keys {
		switch key.Name() {
		case "MODE":
			mode = key.MustString(name)
		case "FILE_NAME":
			logPath = key.MustString(defaults.filename)
			forcePathSeparator(logPath)
			if !filepath.IsAbs(logPath) {
				logPath = path.Join(Log.RootPath, logPath)
			}
		case "FLAGS":
			flags = log.FlagsFromString(key.MustString(defaults.flags))
		case "EXPRESSION":
			expression = key.MustString("")
		case "PREFIX":
			prefix = key.MustString("")
		}
	}

	logConfig := map[string]interface{}{
		"level":           level.String(),
		"expression":      expression,
		"prefix":          prefix,
		"flags":           flags,
		"stacktraceLevel": stacktraceLevel.String(),
	}

	// Generate log configuration.
	switch mode {
	case "console":
		useStderr := sec.Key("STDERR").MustBool(false)
		logConfig["stderr"] = useStderr
		if useStderr {
			logConfig["colorize"] = sec.Key("COLORIZE").MustBool(log.CanColorStderr)
		} else {
			logConfig["colorize"] = sec.Key("COLORIZE").MustBool(log.CanColorStdout)
		}

	case "file":
		if err := os.MkdirAll(path.Dir(logPath), os.ModePerm); err != nil {
			panic(err.Error())
		}

		logConfig["filename"] = logPath + filenameSuffix
		logConfig["rotate"] = sec.Key("LOG_ROTATE").MustBool(true)
		logConfig["maxsize"] = 1 << uint(sec.Key("MAX_SIZE_SHIFT").MustInt(28))
		logConfig["daily"] = sec.Key("DAILY_ROTATE").MustBool(true)
		logConfig["maxdays"] = sec.Key("MAX_DAYS").MustInt(7)
		logConfig["compress"] = sec.Key("COMPRESS").MustBool(true)
		logConfig["compressionLevel"] = sec.Key("COMPRESSION_LEVEL").MustInt(-1)
	case "conn":
		logConfig["reconnectOnMsg"] = sec.Key("RECONNECT_ON_MSG").MustBool()
		logConfig["reconnect"] = sec.Key("RECONNECT").MustBool()
		logConfig["net"] = sec.Key("PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"})
		logConfig["addr"] = sec.Key("ADDR").MustString(":7020")
	case "smtp":
		logConfig["username"] = sec.Key("USER").MustString("example@example.com")
		logConfig["password"] = sec.Key("PASSWD").MustString("******")
		logConfig["host"] = sec.Key("HOST").MustString("127.0.0.1:25")
		sendTos := strings.Split(sec.Key("RECEIVERS").MustString(""), ",")
		for i, address := range sendTos {
			sendTos[i] = strings.TrimSpace(address)
		}
		logConfig["sendTos"] = sendTos
		logConfig["subject"] = sec.Key("SUBJECT").MustString("Diagnostic message from Gitea")
	}

	logConfig["colorize"] = sec.Key("COLORIZE").MustBool(false)
	byteConfig, err := json.Marshal(logConfig)
	if err != nil {
		log.Error("Failed to marshal log configuration: %v %v", logConfig, err)
		return
	}
	jsonConfig = string(byteConfig)
	return mode, jsonConfig, levelName
}

func generateNamedLogger(rootCfg ConfigProvider, key string, options defaultLogOptions) *LogDescription {
	description := LogDescription{
		Name: key,
	}

	sections := strings.Split(rootCfg.Section("log").Key(strings.ToUpper(key)).MustString(""), ",")

	for i := 0; i < len(sections); i++ {
		sections[i] = strings.TrimSpace(sections[i])
	}

	for _, name := range sections {
		if len(name) == 0 || (name == "console" && options.disableConsole) {
			continue
		}
		sec, err := rootCfg.GetSection("log." + name + "." + key)
		if err != nil {
			sec, _ = rootCfg.NewSection("log." + name + "." + key)
		}

		provider, config, levelName := generateLogConfig(sec, name, options)

		if err := log.NewNamedLogger(key, options.bufferLength, name, provider, config); err != nil {
			// Maybe panic here?
			log.Error("Could not create new named logger: %v", err.Error())
		}

		description.SubLogDescriptions = append(description.SubLogDescriptions, SubLogDescription{
			Name:     name,
			Provider: provider,
			Config:   config,
		})
		log.Info("%s Log: %s(%s:%s)", util.ToTitleCase(key), util.ToTitleCase(name), provider, levelName)
	}

	AddLogDescription(key, &description)

	return &description
}

// initLogFrom initializes logging with settings from configuration provider
func initLogFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("log")
	options := newDefaultLogOptions()
	options.bufferLength = Log.BufferLength

	description := LogDescription{
		Name: log.DEFAULT,
	}

	sections := strings.Split(sec.Key("MODE").MustString("console"), ",")

	useConsole := false
	for _, name := range sections {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == "console" {
			useConsole = true
		}

		sec, err := rootCfg.GetSection("log." + name + ".default")
		if err != nil {
			sec, err = rootCfg.GetSection("log." + name)
			if err != nil {
				sec, _ = rootCfg.NewSection("log." + name)
			}
		}

		provider, config, levelName := generateLogConfig(sec, name, options)
		log.NewLogger(options.bufferLength, name, provider, config)
		description.SubLogDescriptions = append(description.SubLogDescriptions, SubLogDescription{
			Name:     name,
			Provider: provider,
			Config:   config,
		})
		log.Info("Gitea Log Mode: %s(%s:%s)", util.ToTitleCase(name), util.ToTitleCase(provider), levelName)
	}

	AddLogDescription(log.DEFAULT, &description)

	if !useConsole {
		log.Info("According to the configuration, subsequent logs will not be printed to the console")
		if err := log.DelLogger("console"); err != nil {
			log.Fatal("Cannot delete console logger: %v", err)
		}
	}

	// Finally redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))
}

// RestartLogsWithPIDSuffix restarts the logs with a PID suffix on files
func RestartLogsWithPIDSuffix() {
	filenameSuffix = fmt.Sprintf(".%d", os.Getpid())
	InitLogs(false)
}

// InitLogs creates all the log services
func InitLogs(disableConsole bool) {
	initLogFrom(CfgProvider)

	if !Log.DisableRouterLog {
		options := newDefaultLogOptions()
		options.filename = filepath.Join(Log.RootPath, "router.log")
		options.flags = "date,time" // For the router we don't want any prefixed flags
		options.bufferLength = Log.BufferLength
		generateNamedLogger(CfgProvider, "router", options)
	}

	if Log.EnableAccessLog {
		options := newDefaultLogOptions()
		options.filename = filepath.Join(Log.RootPath, "access.log")
		options.flags = "" // For the router we don't want any prefixed flags
		options.bufferLength = Log.BufferLength
		generateNamedLogger(CfgProvider, "access", options)
	}

	initSQLLogFrom(CfgProvider, disableConsole)
}

// InitSQLLog initializes xorm logger setting
func InitSQLLog(disableConsole bool) {
	initSQLLogFrom(CfgProvider, disableConsole)
}

func initSQLLogFrom(rootCfg ConfigProvider, disableConsole bool) {
	if Log.EnableXORMLog {
		options := newDefaultLogOptions()
		options.filename = filepath.Join(Log.RootPath, "xorm.log")
		options.bufferLength = Log.BufferLength
		options.disableConsole = disableConsole

		rootCfg.Section("log").Key("XORM").MustString(",")
		generateNamedLogger(rootCfg, "xorm", options)
	}
}
