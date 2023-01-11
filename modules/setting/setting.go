// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	ini "gopkg.in/ini.v1"
)

// Scheme describes protocol types
type Scheme string

// enumerates all the scheme types
const (
	HTTP     Scheme = "http"
	HTTPS    Scheme = "https"
	FCGI     Scheme = "fcgi"
	FCGIUnix Scheme = "fcgi+unix"
	HTTPUnix Scheme = "http+unix"
)

// LandingPage describes the default page
type LandingPage string

// enumerates all the landing page types
const (
	LandingPageHome          LandingPage = "/"
	LandingPageExplore       LandingPage = "/explore"
	LandingPageOrganizations LandingPage = "/explore/organizations"
	LandingPageLogin         LandingPage = "/user/login"
)

// settings
var (
	// AppVer is the version of the current build of Gitea. It is set in main.go from main.Version.
	AppVer string
	// AppBuiltWith represents a human readable version go runtime build version and build tags. (See main.go formatBuiltWith().)
	AppBuiltWith string
	// AppStartTime store time gitea has started
	AppStartTime time.Time
	// AppName is the Application name, used in the page title.
	// It maps to ini:"APP_NAME"
	AppName string
	// AppURL is the Application ROOT_URL. It always has a '/' suffix
	// It maps to ini:"ROOT_URL"
	AppURL string
	// AppSubURL represents the sub-url mounting point for gitea. It is either "" or starts with '/' and ends without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL string
	// AppPath represents the path to the gitea binary
	AppPath string
	// AppWorkPath is the "working directory" of Gitea. It maps to the environment variable GITEA_WORK_DIR.
	// If that is not set it is the default set here by the linker or failing that the directory of AppPath.
	//
	// AppWorkPath is used as the base path for several other paths.
	AppWorkPath string
	// AppDataPath is the default path for storing data.
	// It maps to ini:"APP_DATA_PATH" in [server] and defaults to AppWorkPath + "/data"
	AppDataPath string
	// LocalURL is the url for locally running applications to contact Gitea. It always has a '/' suffix
	// It maps to ini:"LOCAL_ROOT_URL" in [server]
	LocalURL string
	// AssetVersion holds a opaque value that is used for cache-busting assets
	AssetVersion string

	// Server settings
	Protocol                   Scheme
	UseProxyProtocol           bool // `ini:"USE_PROXY_PROTOCOL"`
	ProxyProtocolTLSBridging   bool //`ini:"PROXY_PROTOCOL_TLS_BRIDGING"`
	ProxyProtocolHeaderTimeout time.Duration
	ProxyProtocolAcceptUnknown bool
	Domain                     string
	HTTPAddr                   string
	HTTPPort                   string
	LocalUseProxyProtocol      bool
	RedirectOtherPort          bool
	RedirectorUseProxyProtocol bool
	PortToRedirect             string
	OfflineMode                bool
	CertFile                   string
	KeyFile                    string
	StaticRootPath             string
	StaticCacheTime            time.Duration
	EnableGzip                 bool
	LandingPageURL             LandingPage
	LandingPageCustom          string
	UnixSocketPermission       uint32
	EnablePprof                bool
	PprofDataPath              string
	EnableAcme                 bool
	AcmeTOS                    bool
	AcmeLiveDirectory          string
	AcmeEmail                  string
	AcmeURL                    string
	AcmeCARoot                 string
	SSLMinimumVersion          string
	SSLMaximumVersion          string
	SSLCurvePreferences        []string
	SSLCipherSuites            []string
	GracefulRestartable        bool
	GracefulHammerTime         time.Duration
	StartupTimeout             time.Duration
	PerWriteTimeout            = 30 * time.Second
	PerWritePerKbTimeout       = 10 * time.Second
	StaticURLPrefix            string
	AbsoluteAssetURL           string

	CSRFCookieName     = "_csrf"
	CSRFCookieHTTPOnly = true

	// Highlight settings are loaded in modules/template/highlight.go

	// Global setting objects
	Cfg           *ini.File
	CustomPath    string // Custom directory path
	CustomConf    string
	PIDFile       = "/run/gitea.pid"
	WritePIDFile  bool
	RunMode       string
	IsProd        bool
	RunUser       string
	IsWindows     bool
	HasRobotsTxt  bool
	EnableSitemap bool
	InternalToken string // internal access token
)

func getAppPath() (string, error) {
	var appPath string
	var err error
	if IsWindows && filepath.IsAbs(os.Args[0]) {
		appPath = filepath.Clean(os.Args[0])
	} else {
		appPath, err = exec.LookPath(os.Args[0])
	}

	if err != nil {
		// FIXME: Once we switch to go 1.19 use !errors.Is(err, exec.ErrDot)
		if !strings.Contains(err.Error(), "cannot run executable found relative to current directory") {
			return "", err
		}
		appPath, err = filepath.Abs(os.Args[0])
	}
	if err != nil {
		return "", err
	}
	appPath, err = filepath.Abs(appPath)
	if err != nil {
		return "", err
	}
	// Note: we don't use path.Dir here because it does not handle case
	//	which path starts with two "/" in Windows: "//psf/Home/..."
	return strings.ReplaceAll(appPath, "\\", "/"), err
}

func getWorkPath(appPath string) string {
	workPath := AppWorkPath

	if giteaWorkPath, ok := os.LookupEnv("GITEA_WORK_DIR"); ok {
		workPath = giteaWorkPath
	}
	if len(workPath) == 0 {
		i := strings.LastIndex(appPath, "/")
		if i == -1 {
			workPath = appPath
		} else {
			workPath = appPath[:i]
		}
	}
	workPath = strings.ReplaceAll(workPath, "\\", "/")
	if !filepath.IsAbs(workPath) {
		log.Info("Provided work path %s is not absolute - will be made absolute against the current working directory", workPath)

		absPath, err := filepath.Abs(workPath)
		if err != nil {
			log.Error("Unable to absolute %s against the current working directory %v. Will absolute against the AppPath %s", workPath, err, appPath)
			workPath = filepath.Join(appPath, workPath)
		} else {
			workPath = absPath
		}
	}
	return strings.ReplaceAll(workPath, "\\", "/")
}

func init() {
	IsWindows = runtime.GOOS == "windows"
	// We can rely on log.CanColorStdout being set properly because modules/log/console_windows.go comes before modules/setting/setting.go lexicographically
	// By default set this logger at Info - we'll change it later but we need to start with something.
	log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "info", "colorize": %t, "stacktraceLevel": "none"}`, log.CanColorStdout))

	var err error
	if AppPath, err = getAppPath(); err != nil {
		log.Fatal("Failed to get app path: %v", err)
	}
	AppWorkPath = getWorkPath(AppPath)
}

func forcePathSeparator(path string) {
	if strings.Contains(path, "\\") {
		log.Fatal("Do not use '\\' or '\\\\' in paths, instead, please use '/' in all places")
	}
}

func createPIDFile(pidPath string) {
	currentPid := os.Getpid()
	if err := os.MkdirAll(filepath.Dir(pidPath), os.ModePerm); err != nil {
		log.Fatal("Failed to create PID folder: %v", err)
	}

	file, err := os.Create(pidPath)
	if err != nil {
		log.Fatal("Failed to create PID file: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.FormatInt(int64(currentPid), 10)); err != nil {
		log.Fatal("Failed to write PID information: %v", err)
	}
}

// SetCustomPathAndConf will set CustomPath and CustomConf with reference to the
// GITEA_CUSTOM environment variable and with provided overrides before stepping
// back to the default
func SetCustomPathAndConf(providedCustom, providedConf, providedWorkPath string) {
	if len(providedWorkPath) != 0 {
		AppWorkPath = filepath.ToSlash(providedWorkPath)
	}
	if giteaCustom, ok := os.LookupEnv("GITEA_CUSTOM"); ok {
		CustomPath = giteaCustom
	}
	if len(providedCustom) != 0 {
		CustomPath = providedCustom
	}
	if len(CustomPath) == 0 {
		CustomPath = path.Join(AppWorkPath, "custom")
	} else if !filepath.IsAbs(CustomPath) {
		CustomPath = path.Join(AppWorkPath, CustomPath)
	}

	if len(providedConf) != 0 {
		CustomConf = providedConf
	}
	if len(CustomConf) == 0 {
		CustomConf = path.Join(CustomPath, "conf/app.ini")
	} else if !filepath.IsAbs(CustomConf) {
		CustomConf = path.Join(CustomPath, CustomConf)
		log.Warn("Using 'custom' directory as relative origin for configuration file: '%s'", CustomConf)
	}
}

// LoadFromExisting initializes setting options from an existing config file (app.ini)
func LoadFromExisting() {
	loadFromConf(false, "")
}

// LoadAllowEmpty initializes setting options, it's also fine that if the config file (app.ini) doesn't exist
func LoadAllowEmpty() {
	loadFromConf(true, "")
}

// LoadForTest initializes setting options for tests
func LoadForTest(extraConfigs ...string) {
	loadFromConf(true, strings.Join(extraConfigs, "\n"))
	if err := PrepareAppDataPath(); err != nil {
		log.Fatal("Can not prepare APP_DATA_PATH: %v", err)
	}
}

func deprecatedSetting(oldSection, oldKey, newSection, newKey string) {
	if Cfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated fallback `[%s]` `%s` present. Use `[%s]` `%s` instead. This fallback will be removed in v1.19.0", oldSection, oldKey, newSection, newKey)
	}
}

// deprecatedSettingDB add a hint that the configuration has been moved to database but still kept in app.ini
func deprecatedSettingDB(oldSection, oldKey string) {
	if Cfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated `[%s]` `%s` present which has been copied to database table sys_setting", oldSection, oldKey)
	}
}

// loadFromConf initializes configuration context.
// NOTE: do not print any log except error.
func loadFromConf(allowEmpty bool, extraConfig string) {
	Cfg = ini.Empty()

	if WritePIDFile && len(PIDFile) > 0 {
		createPIDFile(PIDFile)
	}

	isFile, err := util.IsFile(CustomConf)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", CustomConf, err)
	}
	if isFile {
		if err := Cfg.Append(CustomConf); err != nil {
			log.Fatal("Failed to load custom conf '%s': %v", CustomConf, err)
		}
	} else if !allowEmpty {
		log.Fatal("Unable to find configuration file: %q.\nEnsure you are running in the correct environment or set the correct configuration file with -c.", CustomConf)
	} // else: no config file, a config file might be created at CustomConf later (might not)

	if extraConfig != "" {
		if err = Cfg.Append([]byte(extraConfig)); err != nil {
			log.Fatal("Unable to append more config: %v", err)
		}
	}

	Cfg.NameMapper = ini.SnackCase

	parseServerSetting(Cfg)
	parseSSHSetting(Cfg)
	parseOAuth2Setting(Cfg)
	parseSecuritySetting(Cfg)
	parseAttachmentSetting(Cfg)
	parseLFSSetting(Cfg)
	parseTimeSetting(Cfg)
	parseRepositorySetting(Cfg)
	parsePictureSetting(Cfg)
	parsePackagesSetting(Cfg)
	parseUISetting(Cfg)
	parseAdminSetting(Cfg)
	parseAPISetting(Cfg)
	parseMetricsSetting(Cfg)
	parseI18nSetting(Cfg)
	parseGitSetting(Cfg)
	parseMirrorSetting(Cfg)
	parseMarkupSetting(Cfg)
}

// CreateOrAppendToCustomConf creates or updates the custom config.
// Use the callback to set individual values.
func CreateOrAppendToCustomConf(purpose string, callback func(cfg *ini.File)) {
	if CustomConf == "" {
		log.Error("Custom config path must not be empty")
		return
	}

	cfg := ini.Empty()
	isFile, err := util.IsFile(CustomConf)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", CustomConf, err)
	}
	if isFile {
		if err := cfg.Append(CustomConf); err != nil {
			log.Error("failed to load custom conf %s: %v", CustomConf, err)
			return
		}
	}

	callback(cfg)

	if err := os.MkdirAll(filepath.Dir(CustomConf), os.ModePerm); err != nil {
		log.Fatal("failed to create '%s': %v", CustomConf, err)
		return
	}
	if err := cfg.SaveTo(CustomConf); err != nil {
		log.Fatal("error saving to custom config: %v", err)
	}
	log.Info("Settings for %s saved to: %q", purpose, CustomConf)

	// Change permissions to be more restrictive
	fi, err := os.Stat(CustomConf)
	if err != nil {
		log.Error("Failed to determine current conf file permissions: %v", err)
		return
	}

	if fi.Mode().Perm() > 0o600 {
		if err = os.Chmod(CustomConf, 0o600); err != nil {
			log.Warn("Failed changing conf file permissions to -rw-------. Consider changing them manually.")
		}
	}
}

// LoadSettings initializes the settings for normal start up
func LoadSettings() {
	ParseDBSetting()
	parseServiceSetting(Cfg)
	parseOAuth2ClientSetting(Cfg)
	ParseLogSettings(false)
	parseCacheSetting(Cfg)
	parseSessionSetting(Cfg)
	mustMapSetting(Cfg, "cors", &CORSConfig)
	if CORSConfig.Enabled {
		log.Info("CORS Service Enabled")
	}

	parseMailSettings(Cfg)
	parseProxySetting(Cfg)
	parseWebhookSetting(Cfg)
	parseMigrationsSetting(Cfg)
	parseIndexerSetting(Cfg)
	parseTaskSetting(Cfg)
	ParseQueueSettings()
	mustMapSetting(Cfg, "project", &Project)
	parseMimeTypeMap(Cfg)
	parseFederationSetting(Cfg)
}

// LoadSettingsForInstall initializes the settings for install
func LoadSettingsForInstall() {
	ParseDBSetting()
	parseServiceSetting(Cfg)
	parseMailerSetting(Cfg)
}
