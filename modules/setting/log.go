// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-xorm/core"
	ini "gopkg.in/ini.v1"
)

type defaultLogOptions struct {
	levelName string // LogLevel
	flags     int
	filename  string //path.Join(LogRootPath, "gitea.log")
}

func newDefaultLogOptions() defaultLogOptions {
	return defaultLogOptions{
		levelName: LogLevel,
		flags:     0,
		filename:  filepath.Join(LogRootPath, "gitea.log"),
	}
}

// LogDescription describes a logger
type LogDescription struct {
	Sections []string
	Configs  []string
}

func getLogLevel(section *ini.Section, key string, defaultValue string) string {
	value := section.Key(key).MustString("info")
	return log.FromString(value).String()
}

func generateLogConfig(sec *ini.Section, name string, defaults defaultLogOptions) (mode, config, levelName string) {
	levelName = getLogLevel(sec, "LEVEL", LogLevel)
	level := log.FromString(levelName)
	mode = name
	keys := sec.Keys()
	logPath := defaults.filename
	flags := defaults.flags
	expression := ""
	prefix := ""
	for _, key := range keys {
		switch key.Name() {
		case "MODE":
			mode = key.MustString(name)
		case "FILE_NAME":
			logPath = key.MustString(defaults.filename)
		case "FLAGS":
			flags = key.MustInt(defaults.flags)
		case "EXPRESSION":
			expression = key.MustString("")
		case "PREFIX":
			prefix = key.MustString("")
		}
	}

	jsonConfig := map[string]interface{}{
		"level":      level.String(),
		"expression": expression,
		"prefix":     prefix,
		"flags":      flags,
	}

	// Generate log configuration.
	switch mode {
	case "console":
		// No-op
	case "file":
		if err := os.MkdirAll(path.Dir(logPath), os.ModePerm); err != nil {
			panic(err.Error())
		}

		jsonConfig["filename"] = logPath
		jsonConfig["rotate"] = sec.Key("LOG_ROTATE").MustBool(true)
		jsonConfig["maxsize"] = 1 << uint(sec.Key("MAX_SIZE_SHIFT").MustInt(28))
		jsonConfig["daily"] = sec.Key("DAILY_ROTATE").MustBool(true)
		jsonConfig["maxdays"] = sec.Key("MAX_DAYS").MustInt(7)
	case "conn":
		jsonConfig["reconnectOnMsg"] = sec.Key("RECONNECT_ON_MSG").MustBool()
		jsonConfig["reconnect"] = sec.Key("RECONNECT").MustBool()
		jsonConfig["net"] = sec.Key("PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"})
		jsonConfig["addr"] = sec.Key("ADDR").MustString(":7020")
	case "smtp":
		jsonConfig["username"] = sec.Key("USER").MustString("example@example.com")
		jsonConfig["password"] = sec.Key("PASSWD").MustString("******")
		jsonConfig["host"] = sec.Key("HOST").MustString("127.0.0.1:25")
		jsonConfig["sendTos"] = sec.Key("RECEIVERS").MustString("[]")
		jsonConfig["subject"] = sec.Key("SUBJECT").MustString("Diagnostic message from Gitea")
	case "database":
		jsonConfig["driver"] = sec.Key("DRIVER").String()
		jsonConfig["conn"] = sec.Key("CONN").String()
	}

	byteConfig, err := json.Marshal(jsonConfig)
	if err != nil {
		log.Error(0, "Failed to marshal log configuration: %v %v", jsonConfig, err)
		return
	}
	config = string(byteConfig)
	return
}

func generateNamedLogger(key string, options defaultLogOptions) *LogDescription {
	description := LogDescription{}
	bufferLength := Cfg.Section("log").Key("BUFFER_LEN").MustInt64(10000)

	description.Sections = strings.Split(Cfg.Section("log").Key(strings.ToUpper(key)).MustString(""), ",")
	description.Configs = make([]string, len(description.Sections))

	for i := 0; i < len(description.Sections); i++ {
		description.Sections[i] = strings.TrimSpace(description.Sections[i])
	}

	for i, name := range description.Sections {
		if len(name) == 0 {
			continue
		}
		sec, err := Cfg.GetSection("log." + name + "." + key)
		if err != nil {
			sec, _ = Cfg.NewSection("log." + name + "." + key)
		}

		var levelName, adapter string
		adapter, description.Configs[i], levelName = generateLogConfig(sec, name, options)

		log.NewNamedLogger(key, bufferLength, name, adapter, description.Configs[i])
		log.Info("%s Log: %s(%s:%s)", strings.Title(key), strings.Title(name), adapter, levelName)
	}
	return &description
}

func newMacaronLogService() {
	options := newDefaultLogOptions()
	options.filename = filepath.Join(LogRootPath, "macaron.log")
	generateNamedLogger("macaron", options)
}

func newAccessLogService() {
	EnableAccessLog = Cfg.Section("log").Key("ENABLE_ACCESS_LOG").MustBool(false)
	AccessLogTemplate = Cfg.Section("log").Key("ACCESS_LOG_TEMPLATE").MustString(
		`{{.Ctx.RemoteAddr}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}\" \"{{.Ctx.Req.UserAgent}}"`)
	if EnableAccessLog {
		options := newDefaultLogOptions()
		options.filename = filepath.Join(LogRootPath, "access.log")
		options.flags = -1 // For the router we don't want any prefixed flags
		generateNamedLogger("access", options)
	}
}

func newLogService() {
	log.Info("Gitea v%s%s", AppVer, AppBuiltWith)

	options := newDefaultLogOptions()

	LogDescriptions["default"] = &LogDescription{}

	LogDescriptions["default"].Sections = strings.Split(Cfg.Section("log").Key("MODE").MustString("console"), ",")
	LogDescriptions["default"].Configs = make([]string, len(LogDescriptions["default"].Sections))

	bufferLength := Cfg.Section("log").Key("BUFFER_LEN").MustInt64(10000)

	useConsole := false
	for i := 0; i < len(LogDescriptions["default"].Sections); i++ {
		LogDescriptions["default"].Sections[i] = strings.TrimSpace(LogDescriptions["default"].Sections[i])
		if LogDescriptions["default"].Sections[i] == "console" {
			useConsole = true
		}
	}

	if !useConsole {
		log.DelLogger("console")
	}

	for i, name := range LogDescriptions["default"].Sections {
		if len(name) == 0 {
			continue
		}

		sec, err := Cfg.GetSection("log." + name)
		if err != nil {
			sec, _ = Cfg.NewSection("log." + name)
		}

		var levelName, adapter string
		adapter, LogDescriptions["default"].Configs[i], levelName = generateLogConfig(sec, name, options)
		log.NewLogger(bufferLength, name, adapter, LogDescriptions["default"].Configs[i])
		log.Info("Gitea Log Mode: %s(%s:%s)", strings.Title(name), strings.Title(adapter), levelName)
	}

}

// NewXORMLogService initializes xorm logger service
func NewXORMLogService(disableConsole bool) {
	options := newDefaultLogOptions()
	options.filename = filepath.Join(LogRootPath, "xorm.log")
	bufferLength := Cfg.Section("log").Key("BUFFER_LEN").MustInt64(10000)

	hasXormLogger := false

	logNames := strings.Split(Cfg.Section("log").Key("XORM").MustString(""), ",")
	for _, name := range logNames {
		name = strings.TrimSpace(name)

		if len(name) == 0 || (disableConsole && name == "console") {
			continue
		}

		sec, err := Cfg.GetSection("log." + name + ".xorm")
		if err != nil {
			sec, _ = Cfg.NewSection("log." + name + ".xorm")
		}

		adapter, config, levelName := generateLogConfig(sec, name, options)
		log.NewXORMLogger(bufferLength, name, adapter, config)
		hasXormLogger = true
		log.Info("XORM Log Mode: %s(%s:%s)", strings.Title(name), strings.Title(adapter), levelName)

		var lvl core.LogLevel
		switch levelName {
		case "Trace", "Debug":
			lvl = core.LOG_DEBUG
		case "Info":
			lvl = core.LOG_INFO
		case "Warn":
			lvl = core.LOG_WARNING
		case "Error", "Critical":
			lvl = core.LOG_ERR
		}
		log.XORMLogger.SetLevel(lvl)
	}

	if !hasXormLogger {
		log.DiscardXORMLogger()
	} else {
		log.XORMLogger.ShowSQL(LogSQL)
	}
}
