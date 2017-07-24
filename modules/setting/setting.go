// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	_ "code.gitea.io/gitea/modules/minwinsvc" // import minwinsvc for windows services
	"code.gitea.io/gitea/modules/user"

	"github.com/Unknwon/com"
	"github.com/dgrijalva/jwt-go"
	_ "github.com/go-macaron/cache/memcache" // memcache plugin for cache
	_ "github.com/go-macaron/cache/redis"
	"github.com/go-macaron/session"
	_ "github.com/go-macaron/session/redis" // redis plugin for store session
	"github.com/go-xorm/core"
	ini "gopkg.in/ini.v1"
	"strk.kbt.io/projects/go/libravatar"
)

// Scheme describes protocol types
type Scheme string

// enumerates all the scheme types
const (
	HTTP       Scheme = "http"
	HTTPS      Scheme = "https"
	FCGI       Scheme = "fcgi"
	UnixSocket Scheme = "unix"
)

// LandingPage describes the default page
type LandingPage string

// enumerates all the landing page types
const (
	LandingPageHome    LandingPage = "/"
	LandingPageExplore LandingPage = "/explore"
)

// settings
var (
	// AppVer settings
	AppVer         string
	AppBuiltWith   string
	AppName        string
	AppURL         string
	AppSubURL      string
	AppSubURLDepth int // Number of slashes
	AppPath        string
	AppDataPath    string

	// Server settings
	Protocol             Scheme
	Domain               string
	HTTPAddr             string
	HTTPPort             string
	LocalURL             string
	OfflineMode          bool
	DisableRouterLog     bool
	CertFile             string
	KeyFile              string
	StaticRootPath       string
	EnableGzip           bool
	LandingPageURL       LandingPage
	UnixSocketPermission uint32
	EnablePprof          bool

	SSH = struct {
		Disabled             bool           `ini:"DISABLE_SSH"`
		StartBuiltinServer   bool           `ini:"START_SSH_SERVER"`
		Domain               string         `ini:"SSH_DOMAIN"`
		Port                 int            `ini:"SSH_PORT"`
		ListenHost           string         `ini:"SSH_LISTEN_HOST"`
		ListenPort           int            `ini:"SSH_LISTEN_PORT"`
		RootPath             string         `ini:"SSH_ROOT_PATH"`
		KeyTestPath          string         `ini:"SSH_KEY_TEST_PATH"`
		KeygenPath           string         `ini:"SSH_KEYGEN_PATH"`
		AuthorizedKeysBackup bool           `ini:"SSH_AUTHORIZED_KEYS_BACKUP"`
		MinimumKeySizeCheck  bool           `ini:"-"`
		MinimumKeySizes      map[string]int `ini:"-"`
		ExposeAnonymous      bool           `ini:"SSH_EXPOSE_ANONYMOUS"`
	}{
		Disabled:           false,
		StartBuiltinServer: false,
		Domain:             "",
		Port:               22,
		KeygenPath:         "ssh-keygen",
	}

	LFS struct {
		StartServer     bool   `ini:"LFS_START_SERVER"`
		ContentPath     string `ini:"LFS_CONTENT_PATH"`
		JWTSecretBase64 string `ini:"LFS_JWT_SECRET"`
		JWTSecretBytes  []byte `ini:"-"`
	}

	// Security settings
	InstallLock          bool
	SecretKey            string
	LogInRememberDays    int
	CookieUserName       string
	CookieRememberName   string
	ReverseProxyAuthUser string
	MinPasswordLength    int
	ImportLocalPaths     bool

	// Database settings
	UseSQLite3    bool
	UseMySQL      bool
	UseMSSQL      bool
	UsePostgreSQL bool
	UseTiDB       bool

	// Indexer settings
	Indexer struct {
		IssuePath         string
		UpdateQueueLength int
	}

	// Webhook settings
	Webhook = struct {
		QueueLength    int
		DeliverTimeout int
		SkipTLSVerify  bool
		Types          []string
		PagingNum      int
	}{
		QueueLength:    1000,
		DeliverTimeout: 5,
		SkipTLSVerify:  false,
		PagingNum:      10,
	}

	// Repository settings
	Repository = struct {
		AnsiCharset            string
		ForcePrivate           bool
		MaxCreationLimit       int
		MirrorQueueLength      int
		PullRequestQueueLength int
		PreferredLicenses      []string
		DisableHTTPGit         bool

		// Repository editor settings
		Editor struct {
			LineWrapExtensions   []string
			PreviewableFileModes []string
		} `ini:"-"`

		// Repository upload settings
		Upload struct {
			Enabled      bool
			TempPath     string
			AllowedTypes []string `delim:"|"`
			FileMaxSize  int64
			MaxFiles     int
		} `ini:"-"`

		// Repository local settings
		Local struct {
			LocalCopyPath string
		} `ini:"-"`
	}{
		AnsiCharset:            "",
		ForcePrivate:           false,
		MaxCreationLimit:       -1,
		MirrorQueueLength:      1000,
		PullRequestQueueLength: 1000,
		PreferredLicenses:      []string{"Apache License 2.0,MIT License"},
		DisableHTTPGit:         false,

		// Repository editor settings
		Editor: struct {
			LineWrapExtensions   []string
			PreviewableFileModes []string
		}{
			LineWrapExtensions:   strings.Split(".txt,.md,.markdown,.mdown,.mkd,", ","),
			PreviewableFileModes: []string{"markdown"},
		},

		// Repository upload settings
		Upload: struct {
			Enabled      bool
			TempPath     string
			AllowedTypes []string `delim:"|"`
			FileMaxSize  int64
			MaxFiles     int
		}{
			Enabled:      true,
			TempPath:     "data/tmp/uploads",
			AllowedTypes: []string{},
			FileMaxSize:  3,
			MaxFiles:     5,
		},

		// Repository local settings
		Local: struct {
			LocalCopyPath string
		}{
			LocalCopyPath: "tmp/local-repo",
		},
	}
	RepoRootPath string
	ScriptType   = "bash"

	// UI settings
	UI = struct {
		ExplorePagingNum   int
		IssuePagingNum     int
		FeedMaxCommitNum   int
		ThemeColorMetaTag  string
		MaxDisplayFileSize int64
		ShowUserEmail      bool

		Admin struct {
			UserPagingNum   int
			RepoPagingNum   int
			NoticePagingNum int
			OrgPagingNum    int
		} `ini:"ui.admin"`
		User struct {
			RepoPagingNum int
		} `ini:"ui.user"`
		Meta struct {
			Author      string
			Description string
			Keywords    string
		} `ini:"ui.meta"`
	}{
		ExplorePagingNum:   20,
		IssuePagingNum:     10,
		FeedMaxCommitNum:   5,
		ThemeColorMetaTag:  `#6cc644`,
		MaxDisplayFileSize: 8388608,
		Admin: struct {
			UserPagingNum   int
			RepoPagingNum   int
			NoticePagingNum int
			OrgPagingNum    int
		}{
			UserPagingNum:   50,
			RepoPagingNum:   50,
			NoticePagingNum: 25,
			OrgPagingNum:    50,
		},
		User: struct {
			RepoPagingNum int
		}{
			RepoPagingNum: 15,
		},
		Meta: struct {
			Author      string
			Description string
			Keywords    string
		}{
			Author:      "Gitea - Git with a cup of tea",
			Description: "Gitea (Git with a cup of tea) is a painless self-hosted Git service written in Go",
			Keywords:    "go,git,self-hosted,gitea",
		},
	}

	// Markdown settings
	Markdown = struct {
		EnableHardLineBreak bool
		CustomURLSchemes    []string `ini:"CUSTOM_URL_SCHEMES"`
		FileExtensions      []string
	}{
		EnableHardLineBreak: false,
		FileExtensions:      strings.Split(".md,.markdown,.mdown,.mkd", ","),
	}

	// Admin settings
	Admin struct {
		DisableRegularOrgCreation bool
	}

	// Picture settings
	AvatarUploadPath      string
	GravatarSource        string
	DisableGravatar       bool
	EnableFederatedAvatar bool
	LibravatarService     *libravatar.Libravatar

	// Log settings
	LogRootPath string
	LogModes    []string
	LogConfigs  []string

	// Attachment settings
	AttachmentPath         string
	AttachmentAllowedTypes string
	AttachmentMaxSize      int64
	AttachmentMaxFiles     int
	AttachmentEnabled      bool

	// Time settings
	TimeFormat string

	// Cache settings
	CacheAdapter  string
	CacheInterval int
	CacheConn     string

	// Session settings
	SessionConfig  session.Options
	CSRFCookieName = "_csrf"

	// Cron tasks
	Cron = struct {
		UpdateMirror struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		} `ini:"cron.update_mirrors"`
		RepoHealthCheck struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			Timeout    time.Duration
			Args       []string `delim:" "`
		} `ini:"cron.repo_health_check"`
		CheckRepoStats struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		} `ini:"cron.check_repo_stats"`
		ArchiveCleanup struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			OlderThan  time.Duration
		} `ini:"cron.archive_cleanup"`
		SyncExternalUsers struct {
			Enabled        bool
			RunAtStart     bool
			Schedule       string
			UpdateExisting bool
		} `ini:"cron.sync_external_users"`
	}{
		UpdateMirror: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		}{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 10m",
		},
		RepoHealthCheck: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			Timeout    time.Duration
			Args       []string `delim:" "`
		}{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 24h",
			Timeout:    60 * time.Second,
			Args:       []string{},
		},
		CheckRepoStats: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		}{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
		},
		ArchiveCleanup: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			OlderThan  time.Duration
		}{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
			OlderThan:  24 * time.Hour,
		},
		SyncExternalUsers: struct {
			Enabled        bool
			RunAtStart     bool
			Schedule       string
			UpdateExisting bool
		}{
			Enabled:        true,
			RunAtStart:     false,
			Schedule:       "@every 24h",
			UpdateExisting: true,
		},
	}

	// Git settings
	Git = struct {
		Version                  string `ini:"-"`
		DisableDiffHighlight     bool
		MaxGitDiffLines          int
		MaxGitDiffLineCharacters int
		MaxGitDiffFiles          int
		GCArgs                   []string `delim:" "`
		Timeout                  struct {
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		} `ini:"git.timeout"`
	}{
		DisableDiffHighlight:     false,
		MaxGitDiffLines:          1000,
		MaxGitDiffLineCharacters: 500,
		MaxGitDiffFiles:          100,
		GCArgs:                   []string{},
		Timeout: struct {
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		}{
			Migrate: 600,
			Mirror:  300,
			Clone:   300,
			Pull:    300,
			GC:      60,
		},
	}

	// Mirror settings
	Mirror struct {
		DefaultInterval time.Duration
		MinInterval     time.Duration
	}

	// API settings
	API = struct {
		MaxResponseItems int
	}{
		MaxResponseItems: 50,
	}

	// I18n settings
	Langs     []string
	Names     []string
	dateLangs map[string]string

	// Highlight settings are loaded in modules/template/highlight.go

	// Other settings
	ShowFooterBranding         bool
	ShowFooterVersion          bool
	ShowFooterTemplateLoadTime bool

	// Global setting objects
	Cfg           *ini.File
	CustomPath    string // Custom directory path
	CustomConf    string
	CustomPID     string
	ProdMode      bool
	RunUser       string
	IsWindows     bool
	HasRobotsTxt  bool
	InternalToken string // internal access token
)

// DateLang transforms standard language locale name to corresponding value in datetime plugin.
func DateLang(lang string) string {
	name, ok := dateLangs[lang]
	if ok {
		return name
	}
	return "en"
}

// execPath returns the executable path.
func execPath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(file)
}

func init() {
	IsWindows = runtime.GOOS == "windows"
	log.NewLogger(0, "console", `{"level": 0}`)

	var err error
	if AppPath, err = execPath(); err != nil {
		log.Fatal(4, "Failed to get app path: %v", err)
	}

	// Note: we don't use path.Dir here because it does not handle case
	//	which path starts with two "/" in Windows: "//psf/Home/..."
	AppPath = strings.Replace(AppPath, "\\", "/", -1)
}

// WorkDir returns absolute path of work directory.
func WorkDir() (string, error) {
	wd := os.Getenv("GITEA_WORK_DIR")
	if len(wd) > 0 {
		return wd, nil
	}
	// Use GOGS_WORK_DIR if available, for backward compatibility
	// TODO: drop in 1.1.0 ?
	wd = os.Getenv("GOGS_WORK_DIR")
	if len(wd) > 0 {
		log.Warn(`Usage of GOGS_WORK_DIR is deprecated and will be *removed* in a future release,
please consider changing to GITEA_WORK_DIR`)
		return wd, nil
	}

	i := strings.LastIndex(AppPath, "/")
	if i == -1 {
		return AppPath, nil
	}
	return AppPath[:i], nil
}

func forcePathSeparator(path string) {
	if strings.Contains(path, "\\") {
		log.Fatal(4, "Do not use '\\' or '\\\\' in paths, instead, please use '/' in all places")
	}
}

// IsRunUserMatchCurrentUser returns false if configured run user does not match
// actual user that runs the app. The first return value is the actual user name.
// This check is ignored under Windows since SSH remote login is not the main
// method to login on Windows.
func IsRunUserMatchCurrentUser(runUser string) (string, bool) {
	if IsWindows {
		return "", true
	}

	currentUser := user.CurrentUsername()
	return currentUser, runUser == currentUser
}

func createPIDFile(pidPath string) {
	currentPid := os.Getpid()
	if err := os.MkdirAll(filepath.Dir(pidPath), os.ModePerm); err != nil {
		log.Fatal(4, "Failed to create PID folder: %v", err)
	}

	file, err := os.Create(pidPath)
	if err != nil {
		log.Fatal(4, "Failed to create PID file: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.FormatInt(int64(currentPid), 10)); err != nil {
		log.Fatal(4, "Failed to write PID information: %v", err)
	}
}

// NewContext initializes configuration context.
// NOTE: do not print any log except error.
func NewContext() {
	workDir, err := WorkDir()
	if err != nil {
		log.Fatal(4, "Failed to get work directory: %v", err)
	}

	Cfg = ini.Empty()

	CustomPath = os.Getenv("GITEA_CUSTOM")
	if len(CustomPath) == 0 {
		CustomPath = workDir + "/custom"
	}

	if len(CustomPID) > 0 {
		createPIDFile(CustomPID)
	}

	if len(CustomConf) == 0 {
		CustomConf = CustomPath + "/conf/app.ini"
	} else if !filepath.IsAbs(CustomConf) {
		CustomConf = filepath.Join(workDir, CustomConf)
	}

	if com.IsFile(CustomConf) {
		if err = Cfg.Append(CustomConf); err != nil {
			log.Fatal(4, "Failed to load custom conf '%s': %v", CustomConf, err)
		}
	} else {
		log.Warn("Custom config '%s' not found, ignore this if you're running first time", CustomConf)
	}
	Cfg.NameMapper = ini.AllCapsUnderscore

	homeDir, err := com.HomeDir()
	if err != nil {
		log.Fatal(4, "Failed to get home directory: %v", err)
	}
	homeDir = strings.Replace(homeDir, "\\", "/", -1)

	LogRootPath = Cfg.Section("log").Key("ROOT_PATH").MustString(path.Join(workDir, "log"))
	forcePathSeparator(LogRootPath)

	sec := Cfg.Section("server")
	AppName = Cfg.Section("").Key("APP_NAME").MustString("Gitea: Git with a cup of tea")

	Protocol = HTTP
	if sec.Key("PROTOCOL").String() == "https" {
		Protocol = HTTPS
		CertFile = sec.Key("CERT_FILE").String()
		KeyFile = sec.Key("KEY_FILE").String()
	} else if sec.Key("PROTOCOL").String() == "fcgi" {
		Protocol = FCGI
	} else if sec.Key("PROTOCOL").String() == "unix" {
		Protocol = UnixSocket
		UnixSocketPermissionRaw := sec.Key("UNIX_SOCKET_PERMISSION").MustString("666")
		UnixSocketPermissionParsed, err := strconv.ParseUint(UnixSocketPermissionRaw, 8, 32)
		if err != nil || UnixSocketPermissionParsed > 0777 {
			log.Fatal(4, "Failed to parse unixSocketPermission: %s", UnixSocketPermissionRaw)
		}
		UnixSocketPermission = uint32(UnixSocketPermissionParsed)
	}
	Domain = sec.Key("DOMAIN").MustString("localhost")
	HTTPAddr = sec.Key("HTTP_ADDR").MustString("0.0.0.0")
	HTTPPort = sec.Key("HTTP_PORT").MustString("3000")

	defaultAppURL := string(Protocol) + "://" + Domain
	if (Protocol == HTTP && HTTPPort != "80") || (Protocol == HTTPS && HTTPPort != "443") {
		defaultAppURL += ":" + HTTPPort
	}
	AppURL = sec.Key("ROOT_URL").MustString(defaultAppURL)
	AppURL = strings.TrimRight(AppURL, "/") + "/"

	// Check if has app suburl.
	url, err := url.Parse(AppURL)
	if err != nil {
		log.Fatal(4, "Invalid ROOT_URL '%s': %s", AppURL, err)
	}
	// Suburl should start with '/' and end without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL = strings.TrimSuffix(url.Path, "/")
	AppSubURLDepth = strings.Count(AppSubURL, "/")

	LocalURL = sec.Key("LOCAL_ROOT_URL").MustString(string(Protocol) + "://localhost:" + HTTPPort + "/")
	OfflineMode = sec.Key("OFFLINE_MODE").MustBool()
	DisableRouterLog = sec.Key("DISABLE_ROUTER_LOG").MustBool()
	StaticRootPath = sec.Key("STATIC_ROOT_PATH").MustString(workDir)
	AppDataPath = sec.Key("APP_DATA_PATH").MustString("data")
	EnableGzip = sec.Key("ENABLE_GZIP").MustBool()
	EnablePprof = sec.Key("ENABLE_PPROF").MustBool(false)

	switch sec.Key("LANDING_PAGE").MustString("home") {
	case "explore":
		LandingPageURL = LandingPageExplore
	default:
		LandingPageURL = LandingPageHome
	}

	if len(SSH.Domain) == 0 {
		SSH.Domain = Domain
	}
	SSH.RootPath = path.Join(homeDir, ".ssh")
	SSH.KeyTestPath = os.TempDir()
	if err = Cfg.Section("server").MapTo(&SSH); err != nil {
		log.Fatal(4, "Failed to map SSH settings: %v", err)
	}

	SSH.KeygenPath = sec.Key("SSH_KEYGEN_PATH").MustString("ssh-keygen")
	SSH.Port = sec.Key("SSH_PORT").MustInt(22)
	SSH.ListenPort = sec.Key("SSH_LISTEN_PORT").MustInt(SSH.Port)

	// When disable SSH, start builtin server value is ignored.
	if SSH.Disabled {
		SSH.StartBuiltinServer = false
	}

	if !SSH.Disabled && !SSH.StartBuiltinServer {
		if err := os.MkdirAll(SSH.RootPath, 0700); err != nil {
			log.Fatal(4, "Failed to create '%s': %v", SSH.RootPath, err)
		} else if err = os.MkdirAll(SSH.KeyTestPath, 0644); err != nil {
			log.Fatal(4, "Failed to create '%s': %v", SSH.KeyTestPath, err)
		}
	}

	SSH.MinimumKeySizeCheck = sec.Key("MINIMUM_KEY_SIZE_CHECK").MustBool()
	SSH.MinimumKeySizes = map[string]int{}
	minimumKeySizes := Cfg.Section("ssh.minimum_key_sizes").Keys()
	for _, key := range minimumKeySizes {
		if key.MustInt() != -1 {
			SSH.MinimumKeySizes[strings.ToLower(key.Name())] = key.MustInt()
		}
	}
	SSH.AuthorizedKeysBackup = sec.Key("SSH_AUTHORIZED_KEYS_BACKUP").MustBool(true)
	SSH.ExposeAnonymous = sec.Key("SSH_EXPOSE_ANONYMOUS").MustBool(false)

	if err = Cfg.Section("server").MapTo(&LFS); err != nil {
		log.Fatal(4, "Failed to map LFS settings: %v", err)
	}

	if LFS.StartServer {

		if err := os.MkdirAll(LFS.ContentPath, 0700); err != nil {
			log.Fatal(4, "Failed to create '%s': %v", LFS.ContentPath, err)
		}

		LFS.JWTSecretBytes = make([]byte, 32)
		n, err := base64.RawURLEncoding.Decode(LFS.JWTSecretBytes, []byte(LFS.JWTSecretBase64))

		if err != nil || n != 32 {
			//Generate new secret and save to config

			_, err := io.ReadFull(rand.Reader, LFS.JWTSecretBytes)

			if err != nil {
				log.Fatal(4, "Error reading random bytes: %v", err)
			}

			LFS.JWTSecretBase64 = base64.RawURLEncoding.EncodeToString(LFS.JWTSecretBytes)

			// Save secret
			cfg := ini.Empty()
			if com.IsFile(CustomConf) {
				// Keeps custom settings if there is already something.
				if err := cfg.Append(CustomConf); err != nil {
					log.Error(4, "Failed to load custom conf '%s': %v", CustomConf, err)
				}
			}

			cfg.Section("server").Key("LFS_JWT_SECRET").SetValue(LFS.JWTSecretBase64)

			if err := os.MkdirAll(filepath.Dir(CustomConf), os.ModePerm); err != nil {
				log.Fatal(4, "Failed to create '%s': %v", CustomConf, err)
			}
			if err := cfg.SaveTo(CustomConf); err != nil {
				log.Fatal(4, "Error saving generated JWT Secret to custom config: %v", err)
				return
			}
		}

		//Disable LFS client hooks if installed for the current OS user
		//Needs at least git v2.1.2

		binVersion, err := git.BinVersion()
		if err != nil {
			log.Fatal(4, "Error retrieving git version: %v", err)
		}

		splitVersion := strings.SplitN(binVersion, ".", 3)

		majorVersion, err := strconv.ParseUint(splitVersion[0], 10, 64)
		if err != nil {
			log.Fatal(4, "Error parsing git major version: %v", err)
		}
		minorVersion, err := strconv.ParseUint(splitVersion[1], 10, 64)
		if err != nil {
			log.Fatal(4, "Error parsing git minor version: %v", err)
		}
		revisionVersion, err := strconv.ParseUint(splitVersion[2], 10, 64)
		if err != nil {
			log.Fatal(4, "Error parsing git revision version: %v", err)
		}

		if !((majorVersion > 2) || (majorVersion == 2 && minorVersion > 1) ||
			(majorVersion == 2 && minorVersion == 1 && revisionVersion >= 2)) {

			LFS.StartServer = false
			log.Error(4, "LFS server support needs at least Git v2.1.2")

		} else {

			git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "filter.lfs.required=",
				"-c", "filter.lfs.smudge=", "-c", "filter.lfs.clean=")

		}
	}

	sec = Cfg.Section("security")
	InstallLock = sec.Key("INSTALL_LOCK").MustBool(false)
	SecretKey = sec.Key("SECRET_KEY").MustString("!#@FDEWREWR&*(")
	LogInRememberDays = sec.Key("LOGIN_REMEMBER_DAYS").MustInt(7)
	CookieUserName = sec.Key("COOKIE_USERNAME").MustString("gitea_awesome")
	CookieRememberName = sec.Key("COOKIE_REMEMBER_NAME").MustString("gitea_incredible")
	ReverseProxyAuthUser = sec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")
	MinPasswordLength = sec.Key("MIN_PASSWORD_LENGTH").MustInt(6)
	ImportLocalPaths = sec.Key("IMPORT_LOCAL_PATHS").MustBool(false)
	InternalToken = sec.Key("INTERNAL_TOKEN").String()
	if len(InternalToken) == 0 {
		secretBytes := make([]byte, 32)
		_, err := io.ReadFull(rand.Reader, secretBytes)
		if err != nil {
			log.Fatal(4, "Error reading random bytes: %v", err)
		}

		secretKey := base64.RawURLEncoding.EncodeToString(secretBytes)

		now := time.Now()
		InternalToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"nbf": now.Unix(),
		}).SignedString([]byte(secretKey))

		if err != nil {
			log.Fatal(4, "Error generate internal token: %v", err)
		}

		// Save secret
		cfgSave := ini.Empty()
		if com.IsFile(CustomConf) {
			// Keeps custom settings if there is already something.
			if err := cfgSave.Append(CustomConf); err != nil {
				log.Error(4, "Failed to load custom conf '%s': %v", CustomConf, err)
			}
		}

		cfgSave.Section("security").Key("INTERNAL_TOKEN").SetValue(InternalToken)

		if err := os.MkdirAll(filepath.Dir(CustomConf), os.ModePerm); err != nil {
			log.Fatal(4, "Failed to create '%s': %v", CustomConf, err)
		}
		if err := cfgSave.SaveTo(CustomConf); err != nil {
			log.Fatal(4, "Error saving generated JWT Secret to custom config: %v", err)
		}
	}

	sec = Cfg.Section("attachment")
	AttachmentPath = sec.Key("PATH").MustString(path.Join(AppDataPath, "attachments"))
	if !filepath.IsAbs(AttachmentPath) {
		AttachmentPath = path.Join(workDir, AttachmentPath)
	}
	AttachmentAllowedTypes = strings.Replace(sec.Key("ALLOWED_TYPES").MustString("image/jpeg,image/png,application/zip,application/gzip"), "|", ",", -1)
	AttachmentMaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	AttachmentMaxFiles = sec.Key("MAX_FILES").MustInt(5)
	AttachmentEnabled = sec.Key("ENABLE").MustBool(true)

	TimeFormatKey := Cfg.Section("time").Key("FORMAT").MustString("RFC1123")
	TimeFormat = map[string]string{
		"ANSIC":       time.ANSIC,
		"UnixDate":    time.UnixDate,
		"RubyDate":    time.RubyDate,
		"RFC822":      time.RFC822,
		"RFC822Z":     time.RFC822Z,
		"RFC850":      time.RFC850,
		"RFC1123":     time.RFC1123,
		"RFC1123Z":    time.RFC1123Z,
		"RFC3339":     time.RFC3339,
		"RFC3339Nano": time.RFC3339Nano,
		"Kitchen":     time.Kitchen,
		"Stamp":       time.Stamp,
		"StampMilli":  time.StampMilli,
		"StampMicro":  time.StampMicro,
		"StampNano":   time.StampNano,
	}[TimeFormatKey]
	// When the TimeFormatKey does not exist in the previous map e.g.'2006-01-02 15:04:05'
	if len(TimeFormat) == 0 {
		TimeFormat = TimeFormatKey
		TestTimeFormat, _ := time.Parse(TimeFormat, TimeFormat)
		if TestTimeFormat.Format(time.RFC3339) != "2006-01-02T15:04:05Z" {
			log.Fatal(4, "Can't create time properly, please check your time format has 2006, 01, 02, 15, 04 and 05")
		}
		log.Trace("Custom TimeFormat: %s", TimeFormat)
	}

	RunUser = Cfg.Section("").Key("RUN_USER").MustString(user.CurrentUsername())
	// Does not check run user when the install lock is off.
	if InstallLock {
		currentUser, match := IsRunUserMatchCurrentUser(RunUser)
		if !match {
			log.Fatal(4, "Expect user '%s' but current user is: %s", RunUser, currentUser)
		}
	}

	// Determine and create root git repository path.
	sec = Cfg.Section("repository")
	Repository.DisableHTTPGit = sec.Key("DISABLE_HTTP_GIT").MustBool()
	Repository.MaxCreationLimit = sec.Key("MAX_CREATION_LIMIT").MustInt(-1)
	RepoRootPath = sec.Key("ROOT").MustString(path.Join(homeDir, "gitea-repositories"))
	forcePathSeparator(RepoRootPath)
	if !filepath.IsAbs(RepoRootPath) {
		RepoRootPath = path.Join(workDir, RepoRootPath)
	} else {
		RepoRootPath = path.Clean(RepoRootPath)
	}
	ScriptType = sec.Key("SCRIPT_TYPE").MustString("bash")
	if err = Cfg.Section("repository").MapTo(&Repository); err != nil {
		log.Fatal(4, "Failed to map Repository settings: %v", err)
	} else if err = Cfg.Section("repository.editor").MapTo(&Repository.Editor); err != nil {
		log.Fatal(4, "Failed to map Repository.Editor settings: %v", err)
	} else if err = Cfg.Section("repository.upload").MapTo(&Repository.Upload); err != nil {
		log.Fatal(4, "Failed to map Repository.Upload settings: %v", err)
	} else if err = Cfg.Section("repository.local").MapTo(&Repository.Local); err != nil {
		log.Fatal(4, "Failed to map Repository.Local settings: %v", err)
	}

	if !filepath.IsAbs(Repository.Upload.TempPath) {
		Repository.Upload.TempPath = path.Join(workDir, Repository.Upload.TempPath)
	}

	sec = Cfg.Section("picture")
	AvatarUploadPath = sec.Key("AVATAR_UPLOAD_PATH").MustString(path.Join(AppDataPath, "avatars"))
	forcePathSeparator(AvatarUploadPath)
	if !filepath.IsAbs(AvatarUploadPath) {
		AvatarUploadPath = path.Join(workDir, AvatarUploadPath)
	}
	switch source := sec.Key("GRAVATAR_SOURCE").MustString("gravatar"); source {
	case "duoshuo":
		GravatarSource = "http://gravatar.duoshuo.com/avatar/"
	case "gravatar":
		GravatarSource = "https://secure.gravatar.com/avatar/"
	case "libravatar":
		GravatarSource = "https://seccdn.libravatar.org/avatar/"
	default:
		GravatarSource = source
	}
	DisableGravatar = sec.Key("DISABLE_GRAVATAR").MustBool()
	EnableFederatedAvatar = sec.Key("ENABLE_FEDERATED_AVATAR").MustBool()
	if OfflineMode {
		DisableGravatar = true
		EnableFederatedAvatar = false
	}
	if DisableGravatar {
		EnableFederatedAvatar = false
	}

	if EnableFederatedAvatar {
		LibravatarService = libravatar.New()
		parts := strings.Split(GravatarSource, "/")
		if len(parts) >= 3 {
			if parts[0] == "https:" {
				LibravatarService.SetUseHTTPS(true)
				LibravatarService.SetSecureFallbackHost(parts[2])
			} else {
				LibravatarService.SetUseHTTPS(false)
				LibravatarService.SetFallbackHost(parts[2])
			}
		}
	}

	if err = Cfg.Section("ui").MapTo(&UI); err != nil {
		log.Fatal(4, "Failed to map UI settings: %v", err)
	} else if err = Cfg.Section("markdown").MapTo(&Markdown); err != nil {
		log.Fatal(4, "Failed to map Markdown settings: %v", err)
	} else if err = Cfg.Section("admin").MapTo(&Admin); err != nil {
		log.Fatal(4, "Fail to map Admin settings: %v", err)
	} else if err = Cfg.Section("cron").MapTo(&Cron); err != nil {
		log.Fatal(4, "Failed to map Cron settings: %v", err)
	} else if err = Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal(4, "Failed to map Git settings: %v", err)
	} else if err = Cfg.Section("api").MapTo(&API); err != nil {
		log.Fatal(4, "Failed to map API settings: %v", err)
	}

	sec = Cfg.Section("mirror")
	Mirror.MinInterval = sec.Key("MIN_INTERVAL").MustDuration(10 * time.Minute)
	Mirror.DefaultInterval = sec.Key("DEFAULT_INTERVAL").MustDuration(8 * time.Hour)
	if Mirror.MinInterval.Minutes() < 1 {
		log.Warn("Mirror.MinInterval is too low")
		Mirror.MinInterval = 1 * time.Minute
	}
	if Mirror.DefaultInterval < Mirror.MinInterval {
		log.Warn("Mirror.DefaultInterval is less than Mirror.MinInterval")
		Mirror.DefaultInterval = time.Hour * 8
	}

	Langs = Cfg.Section("i18n").Key("LANGS").Strings(",")
	if len(Langs) == 0 {
		Langs = defaultLangs
	}
	Names = Cfg.Section("i18n").Key("NAMES").Strings(",")
	if len(Names) == 0 {
		Names = defaultLangNames
	}
	dateLangs = Cfg.Section("i18n.datelang").KeysHash()

	ShowFooterBranding = Cfg.Section("other").Key("SHOW_FOOTER_BRANDING").MustBool(false)
	ShowFooterVersion = Cfg.Section("other").Key("SHOW_FOOTER_VERSION").MustBool(true)
	ShowFooterTemplateLoadTime = Cfg.Section("other").Key("SHOW_FOOTER_TEMPLATE_LOAD_TIME").MustBool(true)

	UI.ShowUserEmail = Cfg.Section("ui").Key("SHOW_USER_EMAIL").MustBool(true)

	HasRobotsTxt = com.IsFile(path.Join(CustomPath, "robots.txt"))
}

// Service settings
var Service struct {
	ActiveCodeLives                int
	ResetPwdCodeLives              int
	RegisterEmailConfirm           bool
	DisableRegistration            bool
	ShowRegistrationButton         bool
	RequireSignInView              bool
	EnableNotifyMail               bool
	EnableReverseProxyAuth         bool
	EnableReverseProxyAutoRegister bool
	EnableCaptcha                  bool
	DefaultKeepEmailPrivate        bool
	DefaultAllowCreateOrganization bool
	NoReplyAddress                 string

	// OpenID settings
	EnableOpenIDSignIn bool
	EnableOpenIDSignUp bool
	OpenIDWhitelist    []*regexp.Regexp
	OpenIDBlacklist    []*regexp.Regexp
}

func newService() {
	sec := Cfg.Section("service")
	Service.ActiveCodeLives = sec.Key("ACTIVE_CODE_LIVE_MINUTES").MustInt(180)
	Service.ResetPwdCodeLives = sec.Key("RESET_PASSWD_CODE_LIVE_MINUTES").MustInt(180)
	Service.DisableRegistration = sec.Key("DISABLE_REGISTRATION").MustBool()
	Service.ShowRegistrationButton = sec.Key("SHOW_REGISTRATION_BUTTON").MustBool(!Service.DisableRegistration)
	Service.RequireSignInView = sec.Key("REQUIRE_SIGNIN_VIEW").MustBool()
	Service.EnableReverseProxyAuth = sec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	Service.EnableReverseProxyAutoRegister = sec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	Service.EnableCaptcha = sec.Key("ENABLE_CAPTCHA").MustBool()
	Service.DefaultKeepEmailPrivate = sec.Key("DEFAULT_KEEP_EMAIL_PRIVATE").MustBool()
	Service.DefaultAllowCreateOrganization = sec.Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").MustBool(true)
	Service.NoReplyAddress = sec.Key("NO_REPLY_ADDRESS").MustString("noreply.example.org")

	sec = Cfg.Section("openid")
	Service.EnableOpenIDSignIn = sec.Key("ENABLE_OPENID_SIGNIN").MustBool(false)
	Service.EnableOpenIDSignUp = sec.Key("ENABLE_OPENID_SIGNUP").MustBool(!Service.DisableRegistration && Service.EnableOpenIDSignIn)
	pats := sec.Key("WHITELISTED_URIS").Strings(" ")
	if len(pats) != 0 {
		Service.OpenIDWhitelist = make([]*regexp.Regexp, len(pats))
		for i, p := range pats {
			Service.OpenIDWhitelist[i] = regexp.MustCompilePOSIX(p)
		}
	}
	pats = sec.Key("BLACKLISTED_URIS").Strings(" ")
	if len(pats) != 0 {
		Service.OpenIDBlacklist = make([]*regexp.Regexp, len(pats))
		for i, p := range pats {
			Service.OpenIDBlacklist[i] = regexp.MustCompilePOSIX(p)
		}
	}

}

var logLevels = map[string]string{
	"Trace":    "0",
	"Debug":    "1",
	"Info":     "2",
	"Warn":     "3",
	"Error":    "4",
	"Critical": "5",
}

func newLogService() {
	log.Info("Gitea v%s%s", AppVer, AppBuiltWith)

	LogModes = strings.Split(Cfg.Section("log").Key("MODE").MustString("console"), ",")
	LogConfigs = make([]string, len(LogModes))

	useConsole := false
	for i := 0; i < len(LogModes); i++ {
		LogModes[i] = strings.TrimSpace(LogModes[i])
		if LogModes[i] == "console" {
			useConsole = true
		}
	}

	if !useConsole {
		log.DelLogger("console")
	}

	for i, mode := range LogModes {
		sec, err := Cfg.GetSection("log." + mode)
		if err != nil {
			sec, _ = Cfg.NewSection("log." + mode)
		}

		validLevels := []string{"Trace", "Debug", "Info", "Warn", "Error", "Critical"}
		// Log level.
		levelName := Cfg.Section("log."+mode).Key("LEVEL").In(
			Cfg.Section("log").Key("LEVEL").In("Trace", validLevels),
			validLevels)
		level, ok := logLevels[levelName]
		if !ok {
			log.Fatal(4, "Unknown log level: %s", levelName)
		}

		// Generate log configuration.
		switch mode {
		case "console":
			LogConfigs[i] = fmt.Sprintf(`{"level":%s}`, level)
		case "file":
			logPath := sec.Key("FILE_NAME").MustString(path.Join(LogRootPath, "gitea.log"))
			if err = os.MkdirAll(path.Dir(logPath), os.ModePerm); err != nil {
				panic(err.Error())
			}

			LogConfigs[i] = fmt.Sprintf(
				`{"level":%s,"filename":"%s","rotate":%v,"maxlines":%d,"maxsize":%d,"daily":%v,"maxdays":%d}`, level,
				logPath,
				sec.Key("LOG_ROTATE").MustBool(true),
				sec.Key("MAX_LINES").MustInt(1000000),
				1<<uint(sec.Key("MAX_SIZE_SHIFT").MustInt(28)),
				sec.Key("DAILY_ROTATE").MustBool(true),
				sec.Key("MAX_DAYS").MustInt(7))
		case "conn":
			LogConfigs[i] = fmt.Sprintf(`{"level":%s,"reconnectOnMsg":%v,"reconnect":%v,"net":"%s","addr":"%s"}`, level,
				sec.Key("RECONNECT_ON_MSG").MustBool(),
				sec.Key("RECONNECT").MustBool(),
				sec.Key("PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"}),
				sec.Key("ADDR").MustString(":7020"))
		case "smtp":
			LogConfigs[i] = fmt.Sprintf(`{"level":%s,"username":"%s","password":"%s","host":"%s","sendTos":["%s"],"subject":"%s"}`, level,
				sec.Key("USER").MustString("example@example.com"),
				sec.Key("PASSWD").MustString("******"),
				sec.Key("HOST").MustString("127.0.0.1:25"),
				strings.Replace(sec.Key("RECEIVERS").MustString("example@example.com"), ",", "\",\"", -1),
				sec.Key("SUBJECT").MustString("Diagnostic message from serve"))
		case "database":
			LogConfigs[i] = fmt.Sprintf(`{"level":%s,"driver":"%s","conn":"%s"}`, level,
				sec.Key("DRIVER").String(),
				sec.Key("CONN").String())
		}

		log.NewLogger(Cfg.Section("log").Key("BUFFER_LEN").MustInt64(10000), mode, LogConfigs[i])
		log.Info("Log Mode: %s(%s)", strings.Title(mode), levelName)
	}
}

// NewXORMLogService initializes xorm logger service
func NewXORMLogService(disableConsole bool) {
	logModes := strings.Split(Cfg.Section("log").Key("MODE").MustString("console"), ",")
	var logConfigs string
	for _, mode := range logModes {
		mode = strings.TrimSpace(mode)

		if disableConsole && mode == "console" {
			continue
		}

		sec, err := Cfg.GetSection("log." + mode)
		if err != nil {
			sec, _ = Cfg.NewSection("log." + mode)
		}

		validLevels := []string{"Trace", "Debug", "Info", "Warn", "Error", "Critical"}
		// Log level.
		levelName := Cfg.Section("log."+mode).Key("LEVEL").In(
			Cfg.Section("log").Key("LEVEL").In("Trace", validLevels),
			validLevels)
		level, ok := logLevels[levelName]
		if !ok {
			log.Fatal(4, "Unknown log level: %s", levelName)
		}

		// Generate log configuration.
		switch mode {
		case "console":
			logConfigs = fmt.Sprintf(`{"level":%s}`, level)
		case "file":
			logPath := sec.Key("FILE_NAME").MustString(path.Join(LogRootPath, "xorm.log"))
			if err = os.MkdirAll(path.Dir(logPath), os.ModePerm); err != nil {
				panic(err.Error())
			}
			logPath = filepath.Join(filepath.Dir(logPath), "xorm.log")

			logConfigs = fmt.Sprintf(
				`{"level":%s,"filename":"%s","rotate":%v,"maxlines":%d,"maxsize":%d,"daily":%v,"maxdays":%d}`, level,
				logPath,
				sec.Key("LOG_ROTATE").MustBool(true),
				sec.Key("MAX_LINES").MustInt(1000000),
				1<<uint(sec.Key("MAX_SIZE_SHIFT").MustInt(28)),
				sec.Key("DAILY_ROTATE").MustBool(true),
				sec.Key("MAX_DAYS").MustInt(7))
		case "conn":
			logConfigs = fmt.Sprintf(`{"level":%s,"reconnectOnMsg":%v,"reconnect":%v,"net":"%s","addr":"%s"}`, level,
				sec.Key("RECONNECT_ON_MSG").MustBool(),
				sec.Key("RECONNECT").MustBool(),
				sec.Key("PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"}),
				sec.Key("ADDR").MustString(":7020"))
		case "smtp":
			logConfigs = fmt.Sprintf(`{"level":%s,"username":"%s","password":"%s","host":"%s","sendTos":"%s","subject":"%s"}`, level,
				sec.Key("USER").MustString("example@example.com"),
				sec.Key("PASSWD").MustString("******"),
				sec.Key("HOST").MustString("127.0.0.1:25"),
				sec.Key("RECEIVERS").MustString("[]"),
				sec.Key("SUBJECT").MustString("Diagnostic message from serve"))
		case "database":
			logConfigs = fmt.Sprintf(`{"level":%s,"driver":"%s","conn":"%s"}`, level,
				sec.Key("DRIVER").String(),
				sec.Key("CONN").String())
		}

		log.NewXORMLogger(Cfg.Section("log").Key("BUFFER_LEN").MustInt64(10000), mode, logConfigs)
		if !disableConsole {
			log.Info("XORM Log Mode: %s(%s)", strings.Title(mode), levelName)
		}

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

	if len(logConfigs) == 0 {
		log.DiscardXORMLogger()
	}
}

func newCacheService() {
	CacheAdapter = Cfg.Section("cache").Key("ADAPTER").In("memory", []string{"memory", "redis", "memcache"})
	switch CacheAdapter {
	case "memory":
		CacheInterval = Cfg.Section("cache").Key("INTERVAL").MustInt(60)
	case "redis", "memcache":
		CacheConn = strings.Trim(Cfg.Section("cache").Key("HOST").String(), "\" ")
	default:
		log.Fatal(4, "Unknown cache adapter: %s", CacheAdapter)
	}

	log.Info("Cache Service Enabled")
}

func newSessionService() {
	SessionConfig.Provider = Cfg.Section("session").Key("PROVIDER").In("memory",
		[]string{"memory", "file", "redis", "mysql"})
	SessionConfig.ProviderConfig = strings.Trim(Cfg.Section("session").Key("PROVIDER_CONFIG").String(), "\" ")
	SessionConfig.CookieName = Cfg.Section("session").Key("COOKIE_NAME").MustString("i_like_gitea")
	SessionConfig.CookiePath = AppSubURL
	SessionConfig.Secure = Cfg.Section("session").Key("COOKIE_SECURE").MustBool(false)
	SessionConfig.Gclifetime = Cfg.Section("session").Key("GC_INTERVAL_TIME").MustInt64(86400)
	SessionConfig.Maxlifetime = Cfg.Section("session").Key("SESSION_LIFE_TIME").MustInt64(86400)

	log.Info("Session Service Enabled")
}

// Mailer represents mail service.
type Mailer struct {
	// Mailer
	QueueLength     int
	Name            string
	From            string
	FromEmail       string
	SendAsPlainText bool

	// SMTP sender
	Host              string
	User, Passwd      string
	DisableHelo       bool
	HeloHostname      string
	SkipVerify        bool
	UseCertificate    bool
	CertFile, KeyFile string

	// Sendmail sender
	UseSendmail  bool
	SendmailPath string
}

var (
	// MailService the global mailer
	MailService *Mailer
)

func newMailService() {
	sec := Cfg.Section("mailer")
	// Check mailer setting.
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	MailService = &Mailer{
		QueueLength:     sec.Key("SEND_BUFFER_LEN").MustInt(100),
		Name:            sec.Key("NAME").MustString(AppName),
		SendAsPlainText: sec.Key("SEND_AS_PLAIN_TEXT").MustBool(false),

		Host:           sec.Key("HOST").String(),
		User:           sec.Key("USER").String(),
		Passwd:         sec.Key("PASSWD").String(),
		DisableHelo:    sec.Key("DISABLE_HELO").MustBool(),
		HeloHostname:   sec.Key("HELO_HOSTNAME").String(),
		SkipVerify:     sec.Key("SKIP_VERIFY").MustBool(),
		UseCertificate: sec.Key("USE_CERTIFICATE").MustBool(),
		CertFile:       sec.Key("CERT_FILE").String(),
		KeyFile:        sec.Key("KEY_FILE").String(),

		UseSendmail:  sec.Key("USE_SENDMAIL").MustBool(),
		SendmailPath: sec.Key("SENDMAIL_PATH").MustString("sendmail"),
	}
	MailService.From = sec.Key("FROM").MustString(MailService.User)

	if sec.HasKey("ENABLE_HTML_ALTERNATIVE") {
		log.Warn("ENABLE_HTML_ALTERNATIVE is deprecated, use SEND_AS_PLAIN_TEXT")
		MailService.SendAsPlainText = !sec.Key("ENABLE_HTML_ALTERNATIVE").MustBool(false)
	}

	parsed, err := mail.ParseAddress(MailService.From)
	if err != nil {
		log.Fatal(4, "Invalid mailer.FROM (%s): %v", MailService.From, err)
	}
	MailService.FromEmail = parsed.Address

	log.Info("Mail Service Enabled")
}

func newRegisterMailService() {
	if !Cfg.Section("service").Key("REGISTER_EMAIL_CONFIRM").MustBool() {
		return
	} else if MailService == nil {
		log.Warn("Register Mail Service: Mail Service is not enabled")
		return
	}
	Service.RegisterEmailConfirm = true
	log.Info("Register Mail Service Enabled")
}

func newNotifyMailService() {
	if !Cfg.Section("service").Key("ENABLE_NOTIFY_MAIL").MustBool() {
		return
	} else if MailService == nil {
		log.Warn("Notify Mail Service: Mail Service is not enabled")
		return
	}
	Service.EnableNotifyMail = true
	log.Info("Notify Mail Service Enabled")
}

func newWebhookService() {
	sec := Cfg.Section("webhook")
	Webhook.QueueLength = sec.Key("QUEUE_LENGTH").MustInt(1000)
	Webhook.DeliverTimeout = sec.Key("DELIVER_TIMEOUT").MustInt(5)
	Webhook.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool()
	Webhook.Types = []string{"gitea", "gogs", "slack"}
	Webhook.PagingNum = sec.Key("PAGING_NUM").MustInt(10)
}

// NewServices initializes the services
func NewServices() {
	newService()
	newLogService()
	NewXORMLogService(false)
	newCacheService()
	newSessionService()
	newMailService()
	newRegisterMailService()
	newNotifyMailService()
	newWebhookService()
}
