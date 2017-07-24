// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	"gopkg.in/ini.v1"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/user"
)

const (
	// tplInstall template for installation page
	tplInstall base.TplName = "install"
)

// InstallInit prepare for rendering installation page
func InstallInit(ctx *context.Context) {
	if setting.InstallLock {
		ctx.Handle(404, "Install", errors.New("Installation is prohibited"))
		return
	}

	ctx.Data["Title"] = ctx.Tr("install.install")
	ctx.Data["PageIsInstall"] = true

	dbOpts := []string{"MySQL", "PostgreSQL", "MSSQL"}
	if models.EnableSQLite3 {
		dbOpts = append(dbOpts, "SQLite3")
	}
	if models.EnableTiDB {
		dbOpts = append(dbOpts, "TiDB")
	}
	ctx.Data["DbOptions"] = dbOpts
}

// Install render installation page
func Install(ctx *context.Context) {
	form := auth.InstallForm{}

	// Database settings
	form.DbHost = models.DbCfg.Host
	form.DbUser = models.DbCfg.User
	form.DbName = models.DbCfg.Name
	form.DbPath = models.DbCfg.Path

	ctx.Data["CurDbOption"] = "MySQL"
	switch models.DbCfg.Type {
	case "postgres":
		ctx.Data["CurDbOption"] = "PostgreSQL"
	case "mssql":
		ctx.Data["CurDbOption"] = "MSSQL"
	case "sqlite3":
		if models.EnableSQLite3 {
			ctx.Data["CurDbOption"] = "SQLite3"
		}
	case "tidb":
		if models.EnableTiDB {
			ctx.Data["CurDbOption"] = "TiDB"
		}
	}

	// Application general settings
	form.AppName = setting.AppName
	form.RepoRootPath = setting.RepoRootPath
	form.LFSRootPath = setting.LFS.ContentPath

	// Note(unknown): it's hard for Windows users change a running user,
	// 	so just use current one if config says default.
	if setting.IsWindows && setting.RunUser == "git" {
		form.RunUser = user.CurrentUsername()
	} else {
		form.RunUser = setting.RunUser
	}

	form.Domain = setting.Domain
	form.SSHPort = setting.SSH.Port
	form.HTTPPort = setting.HTTPPort
	form.AppURL = setting.AppURL
	form.LogRootPath = setting.LogRootPath

	// E-mail service settings
	if setting.MailService != nil {
		form.SMTPHost = setting.MailService.Host
		form.SMTPFrom = setting.MailService.From
		form.SMTPUser = setting.MailService.User
	}
	form.RegisterConfirm = setting.Service.RegisterEmailConfirm
	form.MailNotify = setting.Service.EnableNotifyMail

	// Server and other services settings
	form.OfflineMode = setting.OfflineMode
	form.DisableGravatar = setting.DisableGravatar
	form.EnableFederatedAvatar = setting.EnableFederatedAvatar
	form.DisableRegistration = setting.Service.DisableRegistration
	form.EnableCaptcha = setting.Service.EnableCaptcha
	form.RequireSignInView = setting.Service.RequireSignInView
	form.DefaultKeepEmailPrivate = setting.Service.DefaultKeepEmailPrivate
	form.DefaultAllowCreateOrganization = setting.Service.DefaultAllowCreateOrganization
	form.NoReplyAddress = setting.Service.NoReplyAddress

	auth.AssignForm(form, ctx.Data)
	ctx.HTML(200, tplInstall)
}

// InstallPost response for submit install items
func InstallPost(ctx *context.Context, form auth.InstallForm) {
	var err error
	ctx.Data["CurDbOption"] = form.DbType

	if ctx.HasError() {
		if ctx.HasValue("Err_SMTPUser") {
			ctx.Data["Err_SMTP"] = true
		}
		if ctx.HasValue("Err_AdminName") ||
			ctx.HasValue("Err_AdminPasswd") ||
			ctx.HasValue("Err_AdminEmail") {
			ctx.Data["Err_Admin"] = true
		}

		ctx.HTML(200, tplInstall)
		return
	}

	if _, err = exec.LookPath("git"); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.test_git_failed", err), tplInstall, &form)
		return
	}

	// Pass basic check, now test configuration.
	// Test database setting.
	dbTypes := map[string]string{"MySQL": "mysql", "PostgreSQL": "postgres", "MSSQL": "mssql", "SQLite3": "sqlite3", "TiDB": "tidb"}
	models.DbCfg.Type = dbTypes[form.DbType]
	models.DbCfg.Host = form.DbHost
	models.DbCfg.User = form.DbUser
	models.DbCfg.Passwd = form.DbPasswd
	models.DbCfg.Name = form.DbName
	models.DbCfg.SSLMode = form.SSLMode
	models.DbCfg.Path = form.DbPath

	if (models.DbCfg.Type == "sqlite3" || models.DbCfg.Type == "tidb") &&
		len(models.DbCfg.Path) == 0 {
		ctx.Data["Err_DbPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.err_empty_db_path"), tplInstall, &form)
		return
	} else if models.DbCfg.Type == "tidb" &&
		strings.ContainsAny(path.Base(models.DbCfg.Path), ".-") {
		ctx.Data["Err_DbPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.err_invalid_tidb_name"), tplInstall, &form)
		return
	}

	// Set test engine.
	var x *xorm.Engine
	if err = models.NewTestEngine(x); err != nil {
		if strings.Contains(err.Error(), `Unknown database type: sqlite3`) {
			ctx.Data["Err_DbType"] = true
			ctx.RenderWithErr(ctx.Tr("install.sqlite3_not_available", "https://docs.gitea.io/installation/install_from_binary.html"), tplInstall, &form)
		} else {
			ctx.Data["Err_DbSetting"] = true
			ctx.RenderWithErr(ctx.Tr("install.invalid_db_setting", err), tplInstall, &form)
		}
		return
	}

	// Test repository root path.
	form.RepoRootPath = strings.Replace(form.RepoRootPath, "\\", "/", -1)
	if err = os.MkdirAll(form.RepoRootPath, os.ModePerm); err != nil {
		ctx.Data["Err_RepoRootPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_repo_path", err), tplInstall, &form)
		return
	}

	// Test LFS root path if not empty, empty meaning disable LFS
	if form.LFSRootPath != "" {
		form.LFSRootPath = strings.Replace(form.LFSRootPath, "\\", "/", -1)
		if err := os.MkdirAll(form.LFSRootPath, os.ModePerm); err != nil {
			ctx.Data["Err_LFSRootPath"] = true
			ctx.RenderWithErr(ctx.Tr("install.invalid_lfs_path", err), tplInstall, &form)
			return
		}
	}

	// Test log root path.
	form.LogRootPath = strings.Replace(form.LogRootPath, "\\", "/", -1)
	if err = os.MkdirAll(form.LogRootPath, os.ModePerm); err != nil {
		ctx.Data["Err_LogRootPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_log_root_path", err), tplInstall, &form)
		return
	}

	currentUser, match := setting.IsRunUserMatchCurrentUser(form.RunUser)
	if !match {
		ctx.Data["Err_RunUser"] = true
		ctx.RenderWithErr(ctx.Tr("install.run_user_not_match", form.RunUser, currentUser), tplInstall, &form)
		return
	}

	// Check logic loophole between disable self-registration and no admin account.
	if form.DisableRegistration && len(form.AdminName) == 0 {
		ctx.Data["Err_Services"] = true
		ctx.Data["Err_Admin"] = true
		ctx.RenderWithErr(ctx.Tr("install.no_admin_and_disable_registration"), tplInstall, form)
		return
	}

	// Check admin password.
	if len(form.AdminName) > 0 && len(form.AdminPasswd) == 0 {
		ctx.Data["Err_Admin"] = true
		ctx.Data["Err_AdminPasswd"] = true
		ctx.RenderWithErr(ctx.Tr("install.err_empty_admin_password"), tplInstall, form)
		return
	}
	if form.AdminPasswd != form.AdminConfirmPasswd {
		ctx.Data["Err_Admin"] = true
		ctx.Data["Err_AdminPasswd"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplInstall, form)
		return
	}

	if form.AppURL[len(form.AppURL)-1] != '/' {
		form.AppURL += "/"
	}

	// Save settings.
	cfg := ini.Empty()
	if com.IsFile(setting.CustomConf) {
		// Keeps custom settings if there is already something.
		if err = cfg.Append(setting.CustomConf); err != nil {
			log.Error(4, "Failed to load custom conf '%s': %v", setting.CustomConf, err)
		}
	}
	cfg.Section("database").Key("DB_TYPE").SetValue(models.DbCfg.Type)
	cfg.Section("database").Key("HOST").SetValue(models.DbCfg.Host)
	cfg.Section("database").Key("NAME").SetValue(models.DbCfg.Name)
	cfg.Section("database").Key("USER").SetValue(models.DbCfg.User)
	cfg.Section("database").Key("PASSWD").SetValue(models.DbCfg.Passwd)
	cfg.Section("database").Key("SSL_MODE").SetValue(models.DbCfg.SSLMode)
	cfg.Section("database").Key("PATH").SetValue(models.DbCfg.Path)

	cfg.Section("").Key("APP_NAME").SetValue(form.AppName)
	cfg.Section("repository").Key("ROOT").SetValue(form.RepoRootPath)
	cfg.Section("").Key("RUN_USER").SetValue(form.RunUser)
	cfg.Section("server").Key("SSH_DOMAIN").SetValue(form.Domain)
	cfg.Section("server").Key("DOMAIN").SetValue(form.Domain)
	cfg.Section("server").Key("HTTP_PORT").SetValue(form.HTTPPort)
	cfg.Section("server").Key("ROOT_URL").SetValue(form.AppURL)

	if form.SSHPort == 0 {
		cfg.Section("server").Key("DISABLE_SSH").SetValue("true")
	} else {
		cfg.Section("server").Key("DISABLE_SSH").SetValue("false")
		cfg.Section("server").Key("SSH_PORT").SetValue(com.ToStr(form.SSHPort))
	}

	if form.LFSRootPath != "" {
		cfg.Section("server").Key("LFS_START_SERVER").SetValue("true")
		cfg.Section("server").Key("LFS_CONTENT_PATH").SetValue(form.LFSRootPath)
		cfg.Section("server").Key("LFS_JWT_SECRET").SetValue(base.GetRandomBytesAsBase64(32))
	} else {
		cfg.Section("server").Key("LFS_START_SERVER").SetValue("false")
	}

	if len(strings.TrimSpace(form.SMTPHost)) > 0 {
		cfg.Section("mailer").Key("ENABLED").SetValue("true")
		cfg.Section("mailer").Key("HOST").SetValue(form.SMTPHost)
		cfg.Section("mailer").Key("FROM").SetValue(form.SMTPFrom)
		cfg.Section("mailer").Key("USER").SetValue(form.SMTPUser)
		cfg.Section("mailer").Key("PASSWD").SetValue(form.SMTPPasswd)
	} else {
		cfg.Section("mailer").Key("ENABLED").SetValue("false")
	}
	cfg.Section("service").Key("REGISTER_EMAIL_CONFIRM").SetValue(com.ToStr(form.RegisterConfirm))
	cfg.Section("service").Key("ENABLE_NOTIFY_MAIL").SetValue(com.ToStr(form.MailNotify))

	cfg.Section("server").Key("OFFLINE_MODE").SetValue(com.ToStr(form.OfflineMode))
	cfg.Section("picture").Key("DISABLE_GRAVATAR").SetValue(com.ToStr(form.DisableGravatar))
	cfg.Section("picture").Key("ENABLE_FEDERATED_AVATAR").SetValue(com.ToStr(form.EnableFederatedAvatar))
	cfg.Section("service").Key("DISABLE_REGISTRATION").SetValue(com.ToStr(form.DisableRegistration))
	cfg.Section("service").Key("ENABLE_CAPTCHA").SetValue(com.ToStr(form.EnableCaptcha))
	cfg.Section("service").Key("REQUIRE_SIGNIN_VIEW").SetValue(com.ToStr(form.RequireSignInView))
	cfg.Section("service").Key("DEFAULT_KEEP_EMAIL_PRIVATE").SetValue(com.ToStr(form.DefaultKeepEmailPrivate))
	cfg.Section("service").Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").SetValue(com.ToStr(form.DefaultAllowCreateOrganization))
	cfg.Section("service").Key("NO_REPLY_ADDRESS").SetValue(com.ToStr(form.NoReplyAddress))

	cfg.Section("").Key("RUN_MODE").SetValue("prod")

	cfg.Section("session").Key("PROVIDER").SetValue("file")

	cfg.Section("log").Key("MODE").SetValue("file")
	cfg.Section("log").Key("LEVEL").SetValue("Info")
	cfg.Section("log").Key("ROOT_PATH").SetValue(form.LogRootPath)

	cfg.Section("security").Key("INSTALL_LOCK").SetValue("true")
	var secretKey string
	if secretKey, err = base.GetRandomString(10); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.secret_key_failed", err), tplInstall, &form)
		return
	}
	cfg.Section("security").Key("SECRET_KEY").SetValue(secretKey)

	err = os.MkdirAll(filepath.Dir(setting.CustomConf), os.ModePerm)
	if err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	if err = cfg.SaveTo(setting.CustomConf); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	GlobalInit()

	// Create admin account
	if len(form.AdminName) > 0 {
		u := &models.User{
			Name:     form.AdminName,
			Email:    form.AdminEmail,
			Passwd:   form.AdminPasswd,
			IsAdmin:  true,
			IsActive: true,
		}
		if err = models.CreateUser(u); err != nil {
			if !models.IsErrUserAlreadyExist(err) {
				setting.InstallLock = false
				ctx.Data["Err_AdminName"] = true
				ctx.Data["Err_AdminEmail"] = true
				ctx.RenderWithErr(ctx.Tr("install.invalid_admin_setting", err), tplInstall, &form)
				return
			}
			log.Info("Admin account already exist")
			u, _ = models.GetUserByName(u.Name)
		}

		// Auto-login for admin
		if err = ctx.Session.Set("uid", u.ID); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}
		if err = ctx.Session.Set("uname", u.Name); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}
	}

	log.Info("First-time run install finished!")
	ctx.Flash.Success(ctx.Tr("install.install_success"))
	ctx.Redirect(form.AppURL + "user/login")
}
