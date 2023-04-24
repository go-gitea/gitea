// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/mailer"

	"gitea.com/go-chi/session"
)

const tplConfig base.TplName = "admin/config"

// SendTestMail send test mail to confirm mail service is OK
func SendTestMail(ctx *context.Context) {
	email := ctx.FormString("email")
	// Send a test email to the user's email address and redirect back to Config
	if err := mailer.SendTestMail(email); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.test_mail_failed", email, err))
	} else {
		ctx.Flash.Info(ctx.Tr("admin.config.test_mail_sent", email))
	}

	ctx.Redirect(setting.AppSubURL + "/admin/config")
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
	ctx.Data["Title"] = ctx.Tr("admin.config")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminConfig"] = true

	systemSettings, err := system_model.GetAllSettings(ctx)
	if err != nil {
		ctx.ServerError("system_model.GetAllSettings", err)
		return
	}

	// All editable settings from UI
	ctx.Data["SystemSettings"] = systemSettings
	ctx.PageData["adminConfigPage"] = true

	ctx.Data["CustomConf"] = setting.CustomConf
	ctx.Data["AppUrl"] = setting.AppURL
	ctx.Data["AppBuiltWith"] = setting.AppBuiltWith
	ctx.Data["Domain"] = setting.Domain
	ctx.Data["OfflineMode"] = setting.OfflineMode
	ctx.Data["DisableRouterLog"] = setting.Log.DisableRouterLog
	ctx.Data["RunUser"] = setting.RunUser
	ctx.Data["RunMode"] = util.ToTitleCase(setting.RunMode)
	ctx.Data["GitVersion"] = git.VersionInfo()

	ctx.Data["RepoRootPath"] = setting.RepoRootPath
	ctx.Data["CustomRootPath"] = setting.CustomPath
	ctx.Data["StaticRootPath"] = setting.StaticRootPath
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

	type envVar struct {
		Name, Value string
	}

	envVars := map[string]*envVar{}
	if len(os.Getenv("GITEA_WORK_DIR")) > 0 {
		envVars["GITEA_WORK_DIR"] = &envVar{"GITEA_WORK_DIR", os.Getenv("GITEA_WORK_DIR")}
	}
	if len(os.Getenv("GITEA_CUSTOM")) > 0 {
		envVars["GITEA_CUSTOM"] = &envVar{"GITEA_CUSTOM", os.Getenv("GITEA_CUSTOM")}
	}

	ctx.Data["EnvVars"] = envVars
	ctx.Data["Loggers"] = setting.GetLogDescriptions()
	ctx.Data["EnableAccessLog"] = setting.Log.EnableAccessLog
	ctx.Data["AccessLogTemplate"] = setting.Log.AccessLogTemplate
	ctx.Data["DisableRouterLog"] = setting.Log.DisableRouterLog
	ctx.Data["EnableXORMLog"] = setting.Log.EnableXORMLog
	ctx.Data["LogSQL"] = setting.Database.LogSQL

	ctx.HTML(http.StatusOK, tplConfig)
}

func ChangeConfig(ctx *context.Context) {
	key := strings.TrimSpace(ctx.FormString("key"))
	if key == "" {
		ctx.JSON(http.StatusOK, map[string]string{
			"redirect": ctx.Req.URL.String(),
		})
		return
	}
	value := ctx.FormString("value")
	version := ctx.FormInt("version")

	if check, ok := changeConfigChecks[key]; ok {
		if err := check(ctx, value); err != nil {
			log.Warn("refused to set setting: %v", err)
			ctx.JSON(http.StatusOK, map[string]string{
				"err": ctx.Tr("admin.config.set_setting_failed", key),
			})
			return
		}
	}

	if err := system_model.SetSetting(ctx, &system_model.Setting{
		SettingKey:   key,
		SettingValue: value,
		Version:      version,
	}); err != nil {
		log.Error("set setting failed: %v", err)
		ctx.JSON(http.StatusOK, map[string]string{
			"err": ctx.Tr("admin.config.set_setting_failed", key),
		})
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"version": version + 1,
	})
}

var changeConfigChecks = map[string]func(ctx *context.Context, newValue string) error{
	system_model.KeyPictureDisableGravatar: func(_ *context.Context, newValue string) error {
		if v, _ := strconv.ParseBool(newValue); setting.OfflineMode && !v {
			return fmt.Errorf("%q should be true when OFFLINE_MODE is true", system_model.KeyPictureDisableGravatar)
		}
		return nil
	},
	system_model.KeyPictureEnableFederatedAvatar: func(_ *context.Context, newValue string) error {
		if v, _ := strconv.ParseBool(newValue); setting.OfflineMode && v {
			return fmt.Errorf("%q cannot be false when OFFLINE_MODE is true", system_model.KeyPictureEnableFederatedAvatar)
		}
		return nil
	},
}
