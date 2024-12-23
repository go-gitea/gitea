// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
	tplConfigSettings templates.TplName = "admin/config_settings"
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
	for i := 0; i < len(fields); i++ {
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

func ConfigSettings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.config_settings")
	ctx.Data["PageIsAdminConfig"] = true
	ctx.Data["PageIsAdminConfigSettings"] = true
	ctx.Data["DefaultOpenWithEditorAppsString"] = setting.DefaultOpenWithEditorApps().ToTextareaString()
	ctx.HTML(http.StatusOK, tplConfigSettings)
}

func ChangeConfig(ctx *context.Context) {
	key := strings.TrimSpace(ctx.FormString("key"))
	value := ctx.FormString("value")
	cfg := setting.Config()

	marshalBool := func(v string) (string, error) { //nolint:unparam
		if b, _ := strconv.ParseBool(v); b {
			return "true", nil
		}
		return "false", nil
	}
	marshalOpenWithApps := func(value string) (string, error) {
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
		b, err := json.Marshal(openWithEditorApps)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	marshallers := map[string]func(string) (string, error){
		cfg.Picture.DisableGravatar.DynKey():       marshalBool,
		cfg.Picture.EnableFederatedAvatar.DynKey(): marshalBool,
		cfg.Repository.OpenWithEditorApps.DynKey(): marshalOpenWithApps,
	}
	marshaller, hasMarshaller := marshallers[key]
	if !hasMarshaller {
		ctx.JSONError(ctx.Tr("admin.config.set_setting_failed", key))
		return
	}
	marshaledValue, err := marshaller(value)
	if err != nil {
		ctx.JSONError(ctx.Tr("admin.config.set_setting_failed", key))
		return
	}
	if err = system_model.SetSettings(ctx, map[string]string{key: marshaledValue}); err != nil {
		ctx.JSONError(ctx.Tr("admin.config.set_setting_failed", key))
		return
	}

	config.GetDynGetter().InvalidateCache()
	ctx.JSONOK()
}
