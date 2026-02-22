// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/mailer"

	"gitea.com/go-chi/session"
)

const (
	tplConfig         templates.TplName = "admin/config"
	tplConfigSettings templates.TplName = "admin/config_settings/config_settings"

	instanceNoticeMessageMaxLength = 2000
)

// SendTestMail send test mail to confirm mail service is OK
func SendTestMail(ctx *context.Context) {
	email := ctx.FormString("email")
	// Send a test email to the user's email address and redirect back to Config
	if err := mailer.SendTestMail(email); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.test_mail_failed", email, err))
	} else {
		ctx.Flash.Info(ctx.Tr("admin.config.test_mail_sent", email))
	}

	ctx.Redirect(setting.AppSubURL + "/-/admin/config")
}

// TestCache test the cache settings
func TestCache(ctx *context.Context) {
	elapsed, err := cache.Test()
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.cache_test_failed", err))
	} else {
		if elapsed > cache.SlowCacheThreshold {
			ctx.Flash.Warning(ctx.Tr("admin.config.cache_test_slow", elapsed))
		} else {
			ctx.Flash.Info(ctx.Tr("admin.config.cache_test_succeeded", elapsed))
		}
	}

	ctx.Redirect(setting.AppSubURL + "/-/admin/config")
}

func shadowPasswordKV(cfgItem, splitter string) string {
	fields := strings.Split(cfgItem, splitter)
	for i := range fields {
		if strings.HasPrefix(fields[i], "password=") {
			fields[i] = "password=******"
			break
		}
	}
	return strings.Join(fields, splitter)
}

func shadowURL(provider, cfgItem string) string {
	u, err := url.Parse(cfgItem)
	if err != nil {
		log.Error("Shadowing Password for %v failed: %v", provider, err)
		return cfgItem
	}
	if u.User != nil {
		atIdx := strings.Index(cfgItem, "@")
		if atIdx > 0 {
			colonIdx := strings.LastIndex(cfgItem[:atIdx], ":")
			if colonIdx > 0 {
				return cfgItem[:colonIdx+1] + "******" + cfgItem[atIdx:]
			}
		}
	}
	return cfgItem
}

func shadowPassword(provider, cfgItem string) string {
	switch provider {
	case "redis":
		return shadowPasswordKV(cfgItem, ",")
	case "mysql":
		// root:@tcp(localhost:3306)/macaron?charset=utf8
		atIdx := strings.Index(cfgItem, "@")
		if atIdx > 0 {
			colonIdx := strings.Index(cfgItem[:atIdx], ":")
			if colonIdx > 0 {
				return cfgItem[:colonIdx+1] + "******" + cfgItem[atIdx:]
			}
		}
		return cfgItem
	case "postgres":
		// user=jiahuachen dbname=macaron port=5432 sslmode=disable
		if !strings.HasPrefix(cfgItem, "postgres://") {
			return shadowPasswordKV(cfgItem, " ")
		}
		fallthrough
	case "couchbase":
		return shadowURL(provider, cfgItem)
		// postgres://pqgotest:password@localhost/pqgotest?sslmode=verify-full
		// Notice: use shadowURL
	}
	return cfgItem
}

// Config show admin config page
func Config(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.config_summary")
	ctx.Data["PageIsAdminConfig"] = true
	ctx.Data["PageIsAdminConfigSummary"] = true

	ctx.Data["CustomConf"] = setting.CustomConf
	ctx.Data["AppUrl"] = setting.AppURL
	ctx.Data["AppBuiltWith"] = setting.AppBuiltWith
	ctx.Data["Domain"] = setting.Domain
	ctx.Data["OfflineMode"] = setting.OfflineMode
	ctx.Data["RunUser"] = setting.RunUser
	ctx.Data["RunMode"] = util.ToTitleCase(setting.RunMode)
	ctx.Data["GitVersion"] = git.DefaultFeatures().VersionInfo()

	ctx.Data["AppDataPath"] = setting.AppDataPath
	ctx.Data["RepoRootPath"] = setting.RepoRootPath
	ctx.Data["CustomRootPath"] = setting.CustomPath
	ctx.Data["LogRootPath"] = setting.Log.RootPath
	ctx.Data["ScriptType"] = setting.ScriptType
	ctx.Data["ReverseProxyAuthUser"] = setting.ReverseProxyAuthUser
	ctx.Data["ReverseProxyAuthEmail"] = setting.ReverseProxyAuthEmail

	ctx.Data["SSH"] = setting.SSH
	ctx.Data["LFS"] = setting.LFS

	ctx.Data["Service"] = setting.Service
	ctx.Data["DbCfg"] = setting.Database
	ctx.Data["Webhook"] = setting.Webhook
	ctx.Data["MailerEnabled"] = false
	if setting.MailService != nil {
		ctx.Data["MailerEnabled"] = true
		ctx.Data["Mailer"] = setting.MailService
	}

	ctx.Data["CacheAdapter"] = setting.CacheService.Adapter
	ctx.Data["CacheInterval"] = setting.CacheService.Interval

	ctx.Data["CacheConn"] = shadowPassword(setting.CacheService.Adapter, setting.CacheService.Conn)
	ctx.Data["CacheItemTTL"] = setting.CacheService.TTL

	sessionCfg := setting.SessionConfig
	if sessionCfg.Provider == "VirtualSession" {
		var realSession session.Options
		if err := json.Unmarshal([]byte(sessionCfg.ProviderConfig), &realSession); err != nil {
			log.Error("Unable to unmarshall session config for virtual provider config: %s\nError: %v", sessionCfg.ProviderConfig, err)
		}
		sessionCfg.Provider = realSession.Provider
		sessionCfg.ProviderConfig = realSession.ProviderConfig
		sessionCfg.CookieName = realSession.CookieName
		sessionCfg.CookiePath = realSession.CookiePath
		sessionCfg.Gclifetime = realSession.Gclifetime
		sessionCfg.Maxlifetime = realSession.Maxlifetime
		sessionCfg.Secure = realSession.Secure
		sessionCfg.Domain = realSession.Domain
	}
	sessionCfg.ProviderConfig = shadowPassword(sessionCfg.Provider, sessionCfg.ProviderConfig)
	ctx.Data["SessionConfig"] = sessionCfg

	ctx.Data["Git"] = setting.Git
	ctx.Data["AccessLogTemplate"] = setting.Log.AccessLogTemplate
	ctx.Data["LogSQL"] = setting.Database.LogSQL

	ctx.Data["Loggers"] = log.GetManager().DumpLoggers()
	config.GetDynGetter().InvalidateCache()
	prepareStartupProblemsAlert(ctx)

	ctx.HTML(http.StatusOK, tplConfig)
}

func parseDatetimeLocalValue(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	tm, err := time.ParseInLocation("2006-01-02T15:04", raw, setting.DefaultUILocation)
	if err != nil {
		return 0, err
	}
	return tm.Unix(), nil
}

func SetInstanceNotice(ctx *context.Context) {
	saveInstanceNotice := func(instanceNotice setting.InstanceNotice) {
		marshaled, err := json.Marshal(instanceNotice)
		if err != nil {
			ctx.ServerError("Marshal", err)
			return
		}
		if err := system_model.SetSettings(ctx, map[string]string{
			setting.Config().InstanceNotice.Banner.DynKey(): string(marshaled),
		}); err != nil {
			ctx.ServerError("SetSettings", err)
			return
		}
		config.GetDynGetter().InvalidateCache()
	}

	if ctx.FormString("action") == "delete" {
		saveInstanceNotice(setting.DefaultInstanceNotice())
		if ctx.Written() {
			return
		}
		ctx.Flash.Success(ctx.Tr("admin.config.instance_notice.delete_success"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
		return
	}

	enabled := ctx.FormBool("enabled")
	message := strings.TrimSpace(ctx.FormString("message"))
	startTime, err := parseDatetimeLocalValue(strings.TrimSpace(ctx.FormString("start_time")))
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.instance_notice.invalid_time"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
		return
	}
	endTime, err := parseDatetimeLocalValue(strings.TrimSpace(ctx.FormString("end_time")))
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.instance_notice.invalid_time"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
		return
	}
	if enabled && message == "" {
		ctx.Flash.Error(ctx.Tr("admin.config.instance_notice.message_required"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
		return
	}
	if utf8.RuneCountInString(message) > instanceNoticeMessageMaxLength {
		ctx.Flash.Error(ctx.Tr("admin.config.instance_notice.message_too_long", instanceNoticeMessageMaxLength))
		ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
		return
	}
	if startTime > 0 && endTime > 0 && endTime < startTime {
		ctx.Flash.Error(ctx.Tr("admin.config.instance_notice.invalid_time_range"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
		return
	}

	instanceNotice := setting.InstanceNotice{
		Enabled:   enabled,
		Message:   message,
		StartTime: startTime,
		EndTime:   endTime,
	}

	saveInstanceNotice(instanceNotice)
	if ctx.Written() {
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.config.instance_notice.save_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/config/settings#instance-notice")
}

func ConfigSettings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.config_settings")
	ctx.Data["PageIsAdminConfig"] = true
	ctx.Data["PageIsAdminConfigSettings"] = true
	ctx.Data["DefaultOpenWithEditorAppsString"] = setting.DefaultOpenWithEditorApps().ToTextareaString()
	instanceNotice := setting.GetInstanceNotice(ctx)
	ctx.Data["InstanceNotice"] = instanceNotice
	ctx.Data["InstanceNoticeMessageMaxLength"] = instanceNoticeMessageMaxLength
	if instanceNotice.StartTime > 0 {
		ctx.Data["InstanceNoticeStartTime"] = time.Unix(instanceNotice.StartTime, 0).In(setting.DefaultUILocation).Format("2006-01-02T15:04")
	}
	if instanceNotice.EndTime > 0 {
		ctx.Data["InstanceNoticeEndTime"] = time.Unix(instanceNotice.EndTime, 0).In(setting.DefaultUILocation).Format("2006-01-02T15:04")
	}
	ctx.HTML(http.StatusOK, tplConfigSettings)
}

func ChangeConfig(ctx *context.Context) {
	cfg := setting.Config()

	marshalBool := func(v string) ([]byte, error) {
		b, _ := strconv.ParseBool(v)
		return json.Marshal(b)
	}

	marshalString := func(emptyDefault string) func(v string) ([]byte, error) {
		return func(v string) ([]byte, error) {
			return json.Marshal(util.IfZero(v, emptyDefault))
		}
	}

	marshalOpenWithApps := func(value string) ([]byte, error) {
		// TODO: move the block alongside OpenWithEditorAppsType.ToTextareaString
		lines := strings.Split(value, "\n")
		var openWithEditorApps setting.OpenWithEditorAppsType
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			displayName, openURL, ok := strings.Cut(line, "=")
			displayName, openURL = strings.TrimSpace(displayName), strings.TrimSpace(openURL)
			if !ok || displayName == "" || openURL == "" {
				continue
			}
			openWithEditorApps = append(openWithEditorApps, setting.OpenWithEditorApp{
				DisplayName: strings.TrimSpace(displayName),
				OpenURL:     strings.TrimSpace(openURL),
			})
		}
		return json.Marshal(openWithEditorApps)
	}
	marshallers := map[string]func(string) ([]byte, error){
		cfg.Picture.DisableGravatar.DynKey():       marshalBool,
		cfg.Picture.EnableFederatedAvatar.DynKey(): marshalBool,
		cfg.Repository.OpenWithEditorApps.DynKey(): marshalOpenWithApps,
		cfg.Repository.GitGuideRemoteName.DynKey(): marshalString(cfg.Repository.GitGuideRemoteName.DefaultValue()),
	}

	_ = ctx.Req.ParseForm()
	configKeys := ctx.Req.Form["key"]
	configValues := ctx.Req.Form["value"]
	configSettings := map[string]string{}
loop:
	for i, key := range configKeys {
		if i >= len(configValues) {
			ctx.JSONError(ctx.Tr("admin.config.set_setting_failed", key))
			break loop
		}
		value := configValues[i]

		marshaller, hasMarshaller := marshallers[key]
		if !hasMarshaller {
			ctx.JSONError(ctx.Tr("admin.config.set_setting_failed", key))
			break loop
		}

		marshaledValue, err := marshaller(value)
		if err != nil {
			ctx.JSONError(ctx.Tr("admin.config.set_setting_failed", key))
			break loop
		}
		configSettings[key] = string(marshaledValue)
	}
	if ctx.Written() {
		return
	}
	if err := system_model.SetSettings(ctx, configSettings); err != nil {
		ctx.ServerError("SetSettings", err)
		return
	}
	config.GetDynGetter().InvalidateCache()
	ctx.JSONOK()
}
