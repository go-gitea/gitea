// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/user"
	"code.gitea.io/gitea/modules/util"
)

// settings
var (
	// AppVer is the version of the current build of Gitea. It is set in main.go from main.Version.
	AppVer string
	// AppBuiltWith represents a human-readable version go runtime build version and build tags. (See main.go formatBuiltWith().)
	AppBuiltWith string
	// AppStartTime store time gitea has started
	AppStartTime time.Time

	CfgProvider ConfigProvider
	IsWindows   bool

	// IsInTesting indicates whether the testing is running. A lot of unreliable code causes a lot of nonsense error logs during testing
	// TODO: this is only a temporary solution, we should make the test code more reliable
	IsInTesting = false
)

func init() {
	IsWindows = runtime.GOOS == "windows"
	if AppVer == "" {
		AppVer = "dev"
	}

	// We can rely on log.CanColorStdout being set properly because modules/log/console_windows.go comes before modules/setting/setting.go lexicographically
	// By default set this logger at Info - we'll change it later, but we need to start with something.
	log.SetConsoleLogger(log.DEFAULT, "console", log.INFO)
}

// IsRunUserMatchCurrentUser returns false if configured run user does not match
// actual user that runs the app. The first return value is the actual user name.
// This check is ignored under Windows since SSH remote login is not the main
// method to login on Windows.
func IsRunUserMatchCurrentUser(runUser string) (string, bool) {
	if IsWindows || SSH.StartBuiltinServer {
		return "", true
	}

	currentUser := user.CurrentUsername()
	return currentUser, runUser == currentUser
}

// PrepareAppDataPath creates app data directory if necessary
func PrepareAppDataPath() error {
	// FIXME: There are too many calls to MkdirAll in old code. It is incorrect.
	// For example, if someDir=/mnt/vol1/gitea-home/data, if the mount point /mnt/vol1 is not mounted when Gitea runs,
	// then gitea will make new empty directories in /mnt/vol1, all are stored in the root filesystem.
	// The correct behavior should be: creating parent directories is end users' duty. We only create sub-directories in existing parent directories.
	// For quickstart, the parent directories should be created automatically for first startup (eg: a flag or a check of INSTALL_LOCK).
	// Now we can take the first step to do correctly (using Mkdir) in other packages, and prepare the AppDataPath here, then make a refactor in future.

	st, err := os.Stat(AppDataPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(AppDataPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create the APP_DATA_PATH directory: %q, Error: %w", AppDataPath, err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to use APP_DATA_PATH %q. Error: %w", AppDataPath, err)
	}

	if !st.IsDir() /* also works for symlink */ {
		return fmt.Errorf("the APP_DATA_PATH %q is not a directory (or symlink to a directory) and can't be used", AppDataPath)
	}

	return nil
}

func InitCfgProvider(file string) {
	var err error
	if CfgProvider, err = NewConfigProviderFromFile(file); err != nil {
		log.Fatal("Unable to init config provider from %q: %v", file, err)
	}
	CfgProvider.DisableSaving() // do not allow saving the CfgProvider into file, it will be polluted by the "MustXxx" calls
}

func MustInstalled() {
	if !InstallLock {
		log.Fatal(`Unable to load config file for a installed Gitea instance, you should either use "--config" to set your config file (app.ini), or run "gitea web" command to install Gitea.`)
	}
}

func LoadCommonSettings() {
	if err := loadCommonSettingsFrom(CfgProvider); err != nil {
		log.Fatal("Unable to load settings from config: %v", err)
	}
}

// loadCommonSettingsFrom loads common configurations from a configuration provider.
func loadCommonSettingsFrom(cfg ConfigProvider) error {
	// WARNING: don't change the sequence except you know what you are doing.
	loadRunModeFrom(cfg)
	loadLogGlobalFrom(cfg)
	loadServerFrom(cfg)
	loadSSHFrom(cfg)

	mustCurrentRunUserMatch(cfg) // it depends on the SSH config, only non-builtin SSH server requires this check

	loadOAuth2From(cfg)
	loadSecurityFrom(cfg)
	if err := loadAttachmentFrom(cfg); err != nil {
		return err
	}
	if err := loadLFSFrom(cfg); err != nil {
		return err
	}
	loadTimeFrom(cfg)
	loadRepositoryFrom(cfg)
	if err := loadAvatarsFrom(cfg); err != nil {
		return err
	}
	if err := loadRepoAvatarFrom(cfg); err != nil {
		return err
	}
	if err := loadPackagesFrom(cfg); err != nil {
		return err
	}
	if err := loadActionsFrom(cfg); err != nil {
		return err
	}
	loadUIFrom(cfg)
	loadAdminFrom(cfg)
	loadAPIFrom(cfg)
	loadMetricsFrom(cfg)
	loadCamoFrom(cfg)
	loadI18nFrom(cfg)
	loadGitFrom(cfg)
	loadMirrorFrom(cfg)
	loadMarkupFrom(cfg)
	loadGlobalLockFrom(cfg)
	loadOtherFrom(cfg)
	return nil
}

func loadRunModeFrom(rootCfg ConfigProvider) {
	rootSec := rootCfg.Section("")
	RunUser = rootSec.Key("RUN_USER").MustString(user.CurrentUsername())

	// The following is a purposefully undocumented option. Please do not run Gitea as root. It will only cause future headaches.
	// Please don't use root as a bandaid to "fix" something that is broken, instead the broken thing should instead be fixed properly.
	unsafeAllowRunAsRoot := ConfigSectionKeyBool(rootSec, "I_AM_BEING_UNSAFE_RUNNING_AS_ROOT")
	unsafeAllowRunAsRoot = unsafeAllowRunAsRoot || util.OptionalBoolParse(os.Getenv("GITEA_I_AM_BEING_UNSAFE_RUNNING_AS_ROOT")).Value()
	RunMode = os.Getenv("GITEA_RUN_MODE")
	if RunMode == "" {
		RunMode = rootSec.Key("RUN_MODE").MustString("prod")
	}

	// non-dev mode is treated as prod mode, to protect users from accidentally running in dev mode if there is a typo in this value.
	RunMode = strings.ToLower(RunMode)
	if RunMode != "dev" {
		RunMode = "prod"
	}
	IsProd = RunMode != "dev"

	// check if we run as root
	if os.Getuid() == 0 {
		if !unsafeAllowRunAsRoot {
			// Special thanks to VLC which inspired the wording of this messaging.
			log.Fatal("Gitea is not supposed to be run as root. Sorry. If you need to use privileged TCP ports please instead use setcap and the `cap_net_bind_service` permission")
		}
		log.Critical("You are running Gitea using the root user, and have purposely chosen to skip built-in protections around this. You have been warned against this.")
	}
}

// HasInstallLock checks the install-lock in ConfigProvider directly, because sometimes the config file is not loaded into setting variables yet.
func HasInstallLock(rootCfg ConfigProvider) bool {
	return rootCfg.Section("security").Key("INSTALL_LOCK").MustBool(false)
}

func mustCurrentRunUserMatch(rootCfg ConfigProvider) {
	// Does not check run user when the "InstallLock" is off.
	if HasInstallLock(rootCfg) {
		currentUser, match := IsRunUserMatchCurrentUser(RunUser)
		if !match {
			log.Fatal("Expect user '%s' but current user is: %s", RunUser, currentUser)
		}
	}
}

// LoadSettings initializes the settings for normal start up
func LoadSettings() {
	initAllLoggers()

	loadDBSetting(CfgProvider)
	loadServiceFrom(CfgProvider)
	loadOAuth2ClientFrom(CfgProvider)
	loadCacheFrom(CfgProvider)
	loadSessionFrom(CfgProvider)
	loadCorsFrom(CfgProvider)
	loadMailsFrom(CfgProvider)
	loadProxyFrom(CfgProvider)
	loadWebhookFrom(CfgProvider)
	loadMigrationsFrom(CfgProvider)
	loadIndexerFrom(CfgProvider)
	loadTaskFrom(CfgProvider)
	LoadQueueSettings()
	loadProjectFrom(CfgProvider)
	loadMimeTypeMapFrom(CfgProvider)
	loadFederationFrom(CfgProvider)
}

// LoadSettingsForInstall initializes the settings for install
func LoadSettingsForInstall() {
	loadDBSetting(CfgProvider)
	loadServiceFrom(CfgProvider)
	loadMailerFrom(CfgProvider)
}

var configuredPaths = make(map[string]string)

func checkOverlappedPath(name, path string) {
	// TODO: some paths shouldn't overlap (storage.xxx.path), while some could (data path is the base path for storage path)
	if targetName, ok := configuredPaths[path]; ok && targetName != name {
		LogStartupProblem(1, log.ERROR, "Configured path %q is used by %q and %q at the same time. The paths must be unique to prevent data loss.", path, targetName, name)
	}
	configuredPaths[path] = name
}

func PanicInDevOrTesting(msg string, a ...any) {
	if !IsProd || IsInTesting {
		panic(fmt.Sprintf(msg, a...))
	}
}
