// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	db_install "code.gitea.io/gitea/models/db/install"
	"code.gitea.io/gitea/models/migrations"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/user"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/routers/common"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/forms"

	"gitea.com/go-chi/session"
)

const (
	// tplInstall template for installation page
	tplInstall     base.TplName = "install"
	tplPostInstall base.TplName = "post-install"
)

// getSupportedDbTypeNames returns a slice for supported database types and names. The slice is used to keep the order
func getSupportedDbTypeNames() (dbTypeNames []map[string]string) {
	for _, t := range setting.SupportedDatabaseTypes {
		dbTypeNames = append(dbTypeNames, map[string]string{"type": t, "name": setting.DatabaseTypeNames[t]})
	}
	return dbTypeNames
}

// Contexter prepare for rendering installation page
func Contexter() func(next http.Handler) http.Handler {
	rnd := templates.HTMLRenderer()
	dbTypeNames := getSupportedDbTypeNames()
	envConfigKeys := setting.CollectEnvConfigKeys()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := context.NewBaseContext(resp, req)
			defer baseCleanUp()

			ctx := context.NewWebContext(base, rnd, session.GetSession(req))
			ctx.AppendContextValue(context.WebContextKey, ctx)
			ctx.Data.MergeFrom(middleware.CommonTemplateContextData())
			ctx.Data.MergeFrom(middleware.ContextData{
				"Context":        ctx, // TODO: use "ctx" in template and remove this
				"locale":         ctx.Locale,
				"Title":          ctx.Locale.Tr("install.install"),
				"PageIsInstall":  true,
				"DbTypeNames":    dbTypeNames,
				"EnvConfigKeys":  envConfigKeys,
				"CustomConfFile": setting.CustomConf,
				"AllLangs":       translation.AllLangs(),

				"PasswordHashAlgorithms": hash.RecommendedHashAlgorithms,
			})
			next.ServeHTTP(resp, ctx.Req)
		})
	}
}

// Install render installation page
func Install(ctx *context.Context) {
	if setting.InstallLock {
		InstallDone(ctx)
		return
	}

	form := forms.InstallForm{}

	// Database settings
	form.DbHost = setting.Database.Host
	form.DbUser = setting.Database.User
	form.DbPasswd = setting.Database.Passwd
	form.DbName = setting.Database.Name
	form.DbPath = setting.Database.Path
	form.DbSchema = setting.Database.Schema
	form.SSLMode = setting.Database.SSLMode

	curDBType := setting.Database.Type.String()
	var isCurDBTypeSupported bool
	for _, dbType := range setting.SupportedDatabaseTypes {
		if dbType == curDBType {
			isCurDBTypeSupported = true
			break
		}
	}
	if !isCurDBTypeSupported {
		curDBType = "mysql"
	}
	ctx.Data["CurDbType"] = curDBType

	// Application general settings
	form.AppName = setting.AppName
	form.RepoRootPath = setting.RepoRootPath
	form.LFSRootPath = setting.LFS.Storage.Path

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
	form.LogRootPath = setting.Log.RootPath

	// E-mail service settings
	if setting.MailService != nil {
		form.SMTPAddr = setting.MailService.SMTPAddr
		form.SMTPPort = setting.MailService.SMTPPort
		form.SMTPFrom = setting.MailService.From
		form.SMTPUser = setting.MailService.User
		form.SMTPPasswd = setting.MailService.Passwd
	}
	form.RegisterConfirm = setting.Service.RegisterEmailConfirm
	form.MailNotify = setting.Service.EnableNotifyMail

	// Server and other services settings
	form.OfflineMode = setting.OfflineMode
	form.DisableGravatar = setting.DisableGravatar             // when installing, there is no database connection so that given a default value
	form.EnableFederatedAvatar = setting.EnableFederatedAvatar // when installing, there is no database connection so that given a default value

	form.EnableOpenIDSignIn = setting.Service.EnableOpenIDSignIn
	form.EnableOpenIDSignUp = setting.Service.EnableOpenIDSignUp
	form.DisableRegistration = setting.Service.DisableRegistration
	form.AllowOnlyExternalRegistration = setting.Service.AllowOnlyExternalRegistration
	form.EnableCaptcha = setting.Service.EnableCaptcha
	form.RequireSignInView = setting.Service.RequireSignInView
	form.DefaultKeepEmailPrivate = setting.Service.DefaultKeepEmailPrivate
	form.DefaultAllowCreateOrganization = setting.Service.DefaultAllowCreateOrganization
	form.DefaultEnableTimetracking = setting.Service.DefaultEnableTimetracking
	form.NoReplyAddress = setting.Service.NoReplyAddress
	form.PasswordAlgorithm = hash.ConfigHashAlgorithm(setting.PasswordHashAlgo)

	middleware.AssignForm(form, ctx.Data)
	ctx.HTML(http.StatusOK, tplInstall)
}

func checkDatabase(ctx *context.Context, form *forms.InstallForm) bool {
	var err error

	if (setting.Database.Type == "sqlite3") &&
		len(setting.Database.Path) == 0 {
		ctx.Data["Err_DbPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.err_empty_db_path"), tplInstall, form)
		return false
	}

	// Check if the user is trying to re-install in an installed database
	db.UnsetDefaultEngine()
	defer db.UnsetDefaultEngine()

	if err = db.InitEngine(ctx); err != nil {
		if strings.Contains(err.Error(), `Unknown database type: sqlite3`) {
			ctx.Data["Err_DbType"] = true
			ctx.RenderWithErr(ctx.Tr("install.sqlite3_not_available", "https://docs.gitea.com/installation/install-from-binary"), tplInstall, form)
		} else {
			ctx.Data["Err_DbSetting"] = true
			ctx.RenderWithErr(ctx.Tr("install.invalid_db_setting", err), tplInstall, form)
		}
		return false
	}

	err = db_install.CheckDatabaseConnection()
	if err != nil {
		ctx.Data["Err_DbSetting"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_db_setting", err), tplInstall, form)
		return false
	}

	hasPostInstallationUser, err := db_install.HasPostInstallationUsers()
	if err != nil {
		ctx.Data["Err_DbSetting"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_db_table", "user", err), tplInstall, form)
		return false
	}
	dbMigrationVersion, err := db_install.GetMigrationVersion()
	if err != nil {
		ctx.Data["Err_DbSetting"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_db_table", "version", err), tplInstall, form)
		return false
	}

	if hasPostInstallationUser && dbMigrationVersion > 0 {
		log.Error("The database is likely to have been used by Gitea before, database migration version=%d", dbMigrationVersion)
		confirmed := form.ReinstallConfirmFirst && form.ReinstallConfirmSecond && form.ReinstallConfirmThird
		if !confirmed {
			ctx.Data["Err_DbInstalledBefore"] = true
			ctx.RenderWithErr(ctx.Tr("install.reinstall_error"), tplInstall, form)
			return false
		}

		log.Info("User confirmed re-installation of Gitea into a pre-existing database")
	}

	if hasPostInstallationUser || dbMigrationVersion > 0 {
		log.Info("Gitea will be installed in a database with: hasPostInstallationUser=%v, dbMigrationVersion=%v", hasPostInstallationUser, dbMigrationVersion)
	}

	return true
}

// SubmitInstall response for submit install items
func SubmitInstall(ctx *context.Context) {
	if setting.InstallLock {
		InstallDone(ctx)
		return
	}

	var err error

	form := *web.GetForm(ctx).(*forms.InstallForm)

	// fix form values
	if form.AppURL != "" && form.AppURL[len(form.AppURL)-1] != '/' {
		form.AppURL += "/"
	}

	ctx.Data["CurDbType"] = form.DbType

	if ctx.HasError() {
		ctx.Data["Err_SMTP"] = ctx.Data["Err_SMTPUser"] != nil
		ctx.Data["Err_Admin"] = ctx.Data["Err_AdminName"] != nil || ctx.Data["Err_AdminPasswd"] != nil || ctx.Data["Err_AdminEmail"] != nil
		ctx.HTML(http.StatusOK, tplInstall)
		return
	}

	if _, err = exec.LookPath("git"); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.test_git_failed", err), tplInstall, &form)
		return
	}

	// ---- Basic checks are passed, now test configuration.

	// Test database setting.
	setting.Database.Type = setting.DatabaseType(form.DbType)
	setting.Database.Host = form.DbHost
	setting.Database.User = form.DbUser
	setting.Database.Passwd = form.DbPasswd
	setting.Database.Name = form.DbName
	setting.Database.Schema = form.DbSchema
	setting.Database.SSLMode = form.SSLMode
	setting.Database.Path = form.DbPath
	setting.Database.LogSQL = !setting.IsProd

	if !checkDatabase(ctx, &form) {
		return
	}

	// Prepare AppDataPath, it is very important for Gitea
	if err = setting.PrepareAppDataPath(); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.invalid_app_data_path", err), tplInstall, &form)
		return
	}

	// Test repository root path.
	form.RepoRootPath = strings.ReplaceAll(form.RepoRootPath, "\\", "/")
	if err = os.MkdirAll(form.RepoRootPath, os.ModePerm); err != nil {
		ctx.Data["Err_RepoRootPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_repo_path", err), tplInstall, &form)
		return
	}

	// Test LFS root path if not empty, empty meaning disable LFS
	if form.LFSRootPath != "" {
		form.LFSRootPath = strings.ReplaceAll(form.LFSRootPath, "\\", "/")
		if err := os.MkdirAll(form.LFSRootPath, os.ModePerm); err != nil {
			ctx.Data["Err_LFSRootPath"] = true
			ctx.RenderWithErr(ctx.Tr("install.invalid_lfs_path", err), tplInstall, &form)
			return
		}
	}

	// Test log root path.
	form.LogRootPath = strings.ReplaceAll(form.LogRootPath, "\\", "/")
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

	// Check admin user creation
	if len(form.AdminName) > 0 {
		// Ensure AdminName is valid
		if err := user_model.IsUsableUsername(form.AdminName); err != nil {
			ctx.Data["Err_Admin"] = true
			ctx.Data["Err_AdminName"] = true
			if db.IsErrNameReserved(err) {
				ctx.RenderWithErr(ctx.Tr("install.err_admin_name_is_reserved"), tplInstall, form)
				return
			} else if db.IsErrNamePatternNotAllowed(err) {
				ctx.RenderWithErr(ctx.Tr("install.err_admin_name_pattern_not_allowed"), tplInstall, form)
				return
			}
			ctx.RenderWithErr(ctx.Tr("install.err_admin_name_is_invalid"), tplInstall, form)
			return
		}
		// Check Admin email
		if len(form.AdminEmail) == 0 {
			ctx.Data["Err_Admin"] = true
			ctx.Data["Err_AdminEmail"] = true
			ctx.RenderWithErr(ctx.Tr("install.err_empty_admin_email"), tplInstall, form)
			return
		}
		// Check admin password.
		if len(form.AdminPasswd) == 0 {
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
	}

	// Init the engine with migration
	if err = db.InitEngineWithMigration(ctx, migrations.Migrate); err != nil {
		db.UnsetDefaultEngine()
		ctx.Data["Err_DbSetting"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_db_setting", err), tplInstall, &form)
		return
	}

	// Save settings.
	cfg, err := setting.NewConfigProviderFromFile(setting.CustomConf)
	if err != nil {
		log.Error("Failed to load custom conf '%s': %v", setting.CustomConf, err)
	}

	cfg.Section("").Key("APP_NAME").SetValue(form.AppName)
	cfg.Section("").Key("RUN_USER").SetValue(form.RunUser)
	cfg.Section("").Key("WORK_PATH").SetValue(setting.AppWorkPath)
	cfg.Section("").Key("RUN_MODE").SetValue("prod")

	cfg.Section("database").Key("DB_TYPE").SetValue(setting.Database.Type.String())
	cfg.Section("database").Key("HOST").SetValue(setting.Database.Host)
	cfg.Section("database").Key("NAME").SetValue(setting.Database.Name)
	cfg.Section("database").Key("USER").SetValue(setting.Database.User)
	cfg.Section("database").Key("PASSWD").SetValue(setting.Database.Passwd)
	cfg.Section("database").Key("SCHEMA").SetValue(setting.Database.Schema)
	cfg.Section("database").Key("SSL_MODE").SetValue(setting.Database.SSLMode)
	cfg.Section("database").Key("PATH").SetValue(setting.Database.Path)
	cfg.Section("database").Key("LOG_SQL").SetValue("false") // LOG_SQL is rarely helpful

	cfg.Section("repository").Key("ROOT").SetValue(form.RepoRootPath)
	cfg.Section("server").Key("SSH_DOMAIN").SetValue(form.Domain)
	cfg.Section("server").Key("DOMAIN").SetValue(form.Domain)
	cfg.Section("server").Key("HTTP_PORT").SetValue(form.HTTPPort)
	cfg.Section("server").Key("ROOT_URL").SetValue(form.AppURL)
	cfg.Section("server").Key("APP_DATA_PATH").SetValue(setting.AppDataPath)

	if form.SSHPort == 0 {
		cfg.Section("server").Key("DISABLE_SSH").SetValue("true")
	} else {
		cfg.Section("server").Key("DISABLE_SSH").SetValue("false")
		cfg.Section("server").Key("SSH_PORT").SetValue(fmt.Sprint(form.SSHPort))
	}

	if form.LFSRootPath != "" {
		cfg.Section("server").Key("LFS_START_SERVER").SetValue("true")
		cfg.Section("lfs").Key("PATH").SetValue(form.LFSRootPath)
		var lfsJwtSecret string
		if _, lfsJwtSecret, err = generate.NewJwtSecretBase64(); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.lfs_jwt_secret_failed", err), tplInstall, &form)
			return
		}
		cfg.Section("server").Key("LFS_JWT_SECRET").SetValue(lfsJwtSecret)
	} else {
		cfg.Section("server").Key("LFS_START_SERVER").SetValue("false")
	}

	if len(strings.TrimSpace(form.SMTPAddr)) > 0 {
		cfg.Section("mailer").Key("ENABLED").SetValue("true")
		cfg.Section("mailer").Key("SMTP_ADDR").SetValue(form.SMTPAddr)
		cfg.Section("mailer").Key("SMTP_PORT").SetValue(form.SMTPPort)
		cfg.Section("mailer").Key("FROM").SetValue(form.SMTPFrom)
		cfg.Section("mailer").Key("USER").SetValue(form.SMTPUser)
		cfg.Section("mailer").Key("PASSWD").SetValue(form.SMTPPasswd)
	} else {
		cfg.Section("mailer").Key("ENABLED").SetValue("false")
	}
	cfg.Section("service").Key("REGISTER_EMAIL_CONFIRM").SetValue(fmt.Sprint(form.RegisterConfirm))
	cfg.Section("service").Key("ENABLE_NOTIFY_MAIL").SetValue(fmt.Sprint(form.MailNotify))

	cfg.Section("server").Key("OFFLINE_MODE").SetValue(fmt.Sprint(form.OfflineMode))
	if err := system_model.SetSettings(ctx, map[string]string{
		setting.Config().Picture.DisableGravatar.DynKey():       strconv.FormatBool(form.DisableGravatar),
		setting.Config().Picture.EnableFederatedAvatar.DynKey(): strconv.FormatBool(form.EnableFederatedAvatar),
	}); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	cfg.Section("openid").Key("ENABLE_OPENID_SIGNIN").SetValue(fmt.Sprint(form.EnableOpenIDSignIn))
	cfg.Section("openid").Key("ENABLE_OPENID_SIGNUP").SetValue(fmt.Sprint(form.EnableOpenIDSignUp))
	cfg.Section("service").Key("DISABLE_REGISTRATION").SetValue(fmt.Sprint(form.DisableRegistration))
	cfg.Section("service").Key("ALLOW_ONLY_EXTERNAL_REGISTRATION").SetValue(fmt.Sprint(form.AllowOnlyExternalRegistration))
	cfg.Section("service").Key("ENABLE_CAPTCHA").SetValue(fmt.Sprint(form.EnableCaptcha))
	cfg.Section("service").Key("REQUIRE_SIGNIN_VIEW").SetValue(fmt.Sprint(form.RequireSignInView))
	cfg.Section("service").Key("DEFAULT_KEEP_EMAIL_PRIVATE").SetValue(fmt.Sprint(form.DefaultKeepEmailPrivate))
	cfg.Section("service").Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").SetValue(fmt.Sprint(form.DefaultAllowCreateOrganization))
	cfg.Section("service").Key("DEFAULT_ENABLE_TIMETRACKING").SetValue(fmt.Sprint(form.DefaultEnableTimetracking))
	cfg.Section("service").Key("NO_REPLY_ADDRESS").SetValue(fmt.Sprint(form.NoReplyAddress))
	cfg.Section("cron.update_checker").Key("ENABLED").SetValue(fmt.Sprint(form.EnableUpdateChecker))

	cfg.Section("session").Key("PROVIDER").SetValue("file")

	cfg.Section("log").Key("MODE").MustString("console")
	cfg.Section("log").Key("LEVEL").SetValue(setting.Log.Level.String())
	cfg.Section("log").Key("ROOT_PATH").SetValue(form.LogRootPath)

	cfg.Section("repository.pull-request").Key("DEFAULT_MERGE_STYLE").SetValue("merge")

	cfg.Section("repository.signing").Key("DEFAULT_TRUST_MODEL").SetValue("committer")

	cfg.Section("security").Key("INSTALL_LOCK").SetValue("true")

	// the internal token could be read from INTERNAL_TOKEN or INTERNAL_TOKEN_URI (the file is guaranteed to be non-empty)
	// if there is no InternalToken, generate one and save to security.INTERNAL_TOKEN
	if setting.InternalToken == "" {
		var internalToken string
		if internalToken, err = generate.NewInternalToken(); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.internal_token_failed", err), tplInstall, &form)
			return
		}
		cfg.Section("security").Key("INTERNAL_TOKEN").SetValue(internalToken)
	}

	// if there is already a SECRET_KEY, we should not overwrite it, otherwise the encrypted data will not be able to be decrypted
	if setting.SecretKey == "" {
		var secretKey string
		if secretKey, err = generate.NewSecretKey(); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.secret_key_failed", err), tplInstall, &form)
			return
		}
		cfg.Section("security").Key("SECRET_KEY").SetValue(secretKey)
	}

	if len(form.PasswordAlgorithm) > 0 {
		var algorithm *hash.PasswordHashAlgorithm
		setting.PasswordHashAlgo, algorithm = hash.SetDefaultPasswordHashAlgorithm(form.PasswordAlgorithm)
		if algorithm == nil {
			ctx.RenderWithErr(ctx.Tr("install.invalid_password_algorithm"), tplInstall, &form)
			return
		}
		cfg.Section("security").Key("PASSWORD_HASH_ALGO").SetValue(form.PasswordAlgorithm)
	}

	log.Info("Save settings to custom config file %s", setting.CustomConf)

	err = os.MkdirAll(filepath.Dir(setting.CustomConf), os.ModePerm)
	if err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	setting.EnvironmentToConfig(cfg, os.Environ())

	if err = cfg.SaveTo(setting.CustomConf); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	// unset default engine before reload database setting
	db.UnsetDefaultEngine()

	// ---- All checks are passed

	// Reload settings (and re-initialize database connection)
	setting.InitCfgProvider(setting.CustomConf)
	setting.LoadCommonSettings()
	setting.MustInstalled()
	setting.LoadDBSetting()
	if err := common.InitDBEngine(ctx); err != nil {
		log.Fatal("ORM engine initialization failed: %v", err)
	}

	// Create admin account
	if len(form.AdminName) > 0 {
		u := &user_model.User{
			Name:    form.AdminName,
			Email:   form.AdminEmail,
			Passwd:  form.AdminPasswd,
			IsAdmin: true,
		}
		overwriteDefault := &user_model.CreateUserOverwriteOptions{
			IsRestricted: util.OptionalBoolFalse,
			IsActive:     util.OptionalBoolTrue,
		}

		if err = user_model.CreateUser(ctx, u, overwriteDefault); err != nil {
			if !user_model.IsErrUserAlreadyExist(err) {
				setting.InstallLock = false
				ctx.Data["Err_AdminName"] = true
				ctx.Data["Err_AdminEmail"] = true
				ctx.RenderWithErr(ctx.Tr("install.invalid_admin_setting", err), tplInstall, &form)
				return
			}
			log.Info("Admin account already exist")
			u, _ = user_model.GetUserByName(ctx, u.Name)
		}

		nt, token, err := auth_service.CreateAuthTokenForUserID(ctx, u.ID)
		if err != nil {
			ctx.ServerError("CreateAuthTokenForUserID", err)
			return
		}

		ctx.SetSiteCookie(setting.CookieRememberName, nt.ID+":"+token, setting.LogInRememberDays*timeutil.Day)

		// Auto-login for admin
		if err = ctx.Session.Set("uid", u.ID); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}
		if err = ctx.Session.Set("uname", u.Name); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}

		if err = ctx.Session.Release(); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}
	}

	setting.ClearEnvConfigKeys()
	log.Info("First-time run install finished!")
	InstallDone(ctx)

	go func() {
		// Sleep for a while to make sure the user's browser has loaded the post-install page and its assets (images, css, js)
		// What if this duration is not long enough? That's impossible -- if the user can't load the simple page in time, how could they install or use Gitea in the future ....
		time.Sleep(3 * time.Second)

		// Now get the http.Server from this request and shut it down
		// NB: This is not our hammerable graceful shutdown this is http.Server.Shutdown
		srv := ctx.Value(http.ServerContextKey).(*http.Server)
		if err := srv.Shutdown(graceful.GetManager().HammerContext()); err != nil {
			log.Error("Unable to shutdown the install server! Error: %v", err)
		}

		// After the HTTP server for "install" shuts down, the `runWeb()` will continue to run the "normal" server
	}()
}

// InstallDone shows the "post-install" page, makes it easier to develop the page.
// The name is not called as "PostInstall" to avoid misinterpretation as a handler for "POST /install"
func InstallDone(ctx *context.Context) { //nolint
	ctx.HTML(http.StatusOK, tplPostInstall)
}
