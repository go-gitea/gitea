// Copyright 2014 The Gogs Authors. All rights reserved.
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
	"runtime"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/user"
	"github.com/Unknwon/com"
	_ "github.com/go-macaron/cache/memcache" // memcache plugin for cache
	_ "github.com/go-macaron/cache/redis"
	"github.com/go-macaron/session"
	_ "github.com/go-macaron/session/redis" // redis plugin for store session
	_ "github.com/kardianos/minwinsvc"      // import minwinsvc for windows services
	"gopkg.in/ini.v1"
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

	SSH struct {
		Disabled            bool           `ini:"DISABLE_SSH"`
		StartBuiltinServer  bool           `ini:"START_SSH_SERVER"`
		Domain              string         `ini:"SSH_DOMAIN"`
		Port                int            `ini:"SSH_PORT"`
		ListenHost          string         `ini:"SSH_LISTEN_HOST"`
		ListenPort          int            `ini:"SSH_LISTEN_PORT"`
		RootPath            string         `ini:"SSH_ROOT_PATH"`
		KeyTestPath         string         `ini:"SSH_KEY_TEST_PATH"`
		KeygenPath          string         `ini:"SSH_KEYGEN_PATH"`
		MinimumKeySizeCheck bool           `ini:"-"`
		MinimumKeySizes     map[string]int `ini:"-"`
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

	// Database settings
	UseSQLite3    bool
	UseMySQL      bool
	UseMSSQL      bool
	UsePostgreSQL bool
	UseTiDB       bool

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

		Admin struct {
			UserPagingNum   int
			RepoPagingNum   int
			NoticePagingNum int
			OrgPagingNum    int
		} `ini:"ui.admin"`
		User struct {
			RepoPagingNum int
		} `ini:"ui.user"`
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
	}

	// Markdown sttings
	Markdown = struct {
		EnableHardLineBreak bool
		CustomURLSchemes    []string `ini:"CUSTOM_URL_SCHEMES"`
		FileExtensions      []string
	}{
		EnableHardLineBreak: false,
		FileExtensions:      strings.Split(".md,.markdown,.mdown,.mkd", ","),
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
	}{
		UpdateMirror: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		}{
			Schedule: "@every 10m",
		},
		RepoHealthCheck: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			Timeout    time.Duration
			Args       []string `delim:" "`
		}{
			Schedule: "@every 24h",
			Timeout:  60 * time.Second,
			Args:     []string{},
		},
		CheckRepoStats: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		}{
			RunAtStart: true,
			Schedule:   "@every 24h",
		},
	}

	// Git settings
	Git = struct {
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
	Mirror = struct {
		DefaultInterval int
	}{
		DefaultInterval: 8,
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

	// Highlight settings are loaded in modules/template/hightlight.go

	// Other settings
	ShowFooterBranding         bool
	ShowFooterVersion          bool
	ShowFooterTemplateLoadTime bool

	// Global setting objects
	Cfg          *ini.File
	CustomPath   string // Custom directory path
	CustomConf   string
	ProdMode     bool
	RunUser      string
	IsWindows    bool
	HasRobotsTxt bool
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
		log.Fatal(4, "fail to get app path: %v\n", err)
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

// NewContext initializes configuration context.
// NOTE: do not print any log except error.
func NewContext() {
	workDir, err := WorkDir()
	if err != nil {
		log.Fatal(4, "Fail to get work directory: %v", err)
	}

	Cfg = ini.Empty()

	if err != nil {
		log.Fatal(4, "Fail to parse 'app.ini': %v", err)
	}

	CustomPath = os.Getenv("GITEA_CUSTOM")
	if len(CustomPath) == 0 {
		// For backward compatibility
		// TODO: drop in 1.1.0 ?
		CustomPath = os.Getenv("GOGS_CUSTOM")
		if len(CustomPath) == 0 {
			CustomPath = workDir + "/custom"
		} else {
			log.Warn(`Usage of GOGS_CUSTOM is deprecated and will be *removed* in a future release,
please consider changing to GITEA_CUSTOM`)
		}
	}

	if len(CustomConf) == 0 {
		CustomConf = CustomPath + "/conf/app.ini"
	}

	if com.IsFile(CustomConf) {
		if err = Cfg.Append(CustomConf); err != nil {
			log.Fatal(4, "Fail to load custom conf '%s': %v", CustomConf, err)
		}
	} else {
		log.Warn("Custom config '%s' not found, ignore this if you're running first time", CustomConf)
	}
	Cfg.NameMapper = ini.AllCapsUnderscore

	homeDir, err := com.HomeDir()
	if err != nil {
		log.Fatal(4, "Fail to get home directory: %v", err)
	}
	homeDir = strings.Replace(homeDir, "\\", "/", -1)

	LogRootPath = Cfg.Section("log").Key("ROOT_PATH").MustString(path.Join(workDir, "log"))
	forcePathSeparator(LogRootPath)

	sec := Cfg.Section("server")
	AppName = Cfg.Section("").Key("APP_NAME").MustString("Gitea: Git with a cup of tea")
	AppURL = sec.Key("ROOT_URL").MustString("http://localhost:3000/")
	if AppURL[len(AppURL)-1] != '/' {
		AppURL += "/"
	}

	// Check if has app suburl.
	url, err := url.Parse(AppURL)
	if err != nil {
		log.Fatal(4, "Invalid ROOT_URL '%s': %s", AppURL, err)
	}
	// Suburl should start with '/' and end without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL = strings.TrimSuffix(url.Path, "/")
	AppSubURLDepth = strings.Count(AppSubURL, "/")

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
			log.Fatal(4, "Fail to parse unixSocketPermission: %s", UnixSocketPermissionRaw)
		}
		UnixSocketPermission = uint32(UnixSocketPermissionParsed)
	}
	Domain = sec.Key("DOMAIN").MustString("localhost")
	HTTPAddr = sec.Key("HTTP_ADDR").MustString("0.0.0.0")
	HTTPPort = sec.Key("HTTP_PORT").MustString("3000")
	LocalURL = sec.Key("LOCAL_ROOT_URL").MustString(string(Protocol) + "://localhost:" + HTTPPort + "/")
	OfflineMode = sec.Key("OFFLINE_MODE").MustBool()
	DisableRouterLog = sec.Key("DISABLE_ROUTER_LOG").MustBool()
	StaticRootPath = sec.Key("STATIC_ROOT_PATH").MustString(workDir)
	AppDataPath = sec.Key("APP_DATA_PATH").MustString("data")
	EnableGzip = sec.Key("ENABLE_GZIP").MustBool()

	switch sec.Key("LANDING_PAGE").MustString("home") {
	case "explore":
		LandingPageURL = LandingPageExplore
	default:
		LandingPageURL = LandingPageHome
	}

	SSH.RootPath = path.Join(homeDir, ".ssh")
	SSH.KeyTestPath = os.TempDir()
	if err = Cfg.Section("server").MapTo(&SSH); err != nil {
		log.Fatal(4, "Fail to map SSH settings: %v", err)
	}

	SSH.KeygenPath = sec.Key("SSH_KEYGEN_PATH").MustString("ssh-keygen")
	SSH.Port = sec.Key("SSH_PORT").MustInt(22)

	// When disable SSH, start builtin server value is ignored.
	if SSH.Disabled {
		SSH.StartBuiltinServer = false
	}

	if !SSH.Disabled && !SSH.StartBuiltinServer {
		if err := os.MkdirAll(SSH.RootPath, 0700); err != nil {
			log.Fatal(4, "Fail to create '%s': %v", SSH.RootPath, err)
		} else if err = os.MkdirAll(SSH.KeyTestPath, 0644); err != nil {
			log.Fatal(4, "Fail to create '%s': %v", SSH.KeyTestPath, err)
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

	if err = Cfg.Section("server").MapTo(&LFS); err != nil {
		log.Fatal(4, "Fail to map LFS settings: %v", err)
	}

	if LFS.StartServer {

		if err := os.MkdirAll(LFS.ContentPath, 0700); err != nil {
			log.Fatal(4, "Fail to create '%s': %v", LFS.ContentPath, err)
		}

		LFS.JWTSecretBytes = make([]byte, 32)
		n, err := base64.RawURLEncoding.Decode(LFS.JWTSecretBytes, []byte(LFS.JWTSecretBase64))

		if err != nil || n != 32 {
			//Generate new secret and save to config

			_, err := io.ReadFull(rand.Reader, LFS.JWTSecretBytes)

			if err != nil {
				log.Fatal(4, "Error reading random bytes: %s", err)
			}

			LFS.JWTSecretBase64 = base64.RawURLEncoding.EncodeToString(LFS.JWTSecretBytes)

			// Save secret
			cfg := ini.Empty()
			if com.IsFile(CustomConf) {
				// Keeps custom settings if there is already something.
				if err := cfg.Append(CustomConf); err != nil {
					log.Error(4, "Fail to load custom conf '%s': %v", CustomConf, err)
				}
			}

			cfg.Section("server").Key("LFS_JWT_SECRET").SetValue(LFS.JWTSecretBase64)

			os.MkdirAll(filepath.Dir(CustomConf), os.ModePerm)
			if err := cfg.SaveTo(CustomConf); err != nil {
				log.Fatal(4, "Error saving generated JWT Secret to custom config: %v", err)
				return
			}
		}

		//Disable LFS client hooks if installed for the current OS user
		//Needs at least git v2.1.2

		binVersion, err := git.BinVersion()
		if err != nil {
			log.Fatal(4, "Error retrieving git version: %s", err)
		}

		splitVersion := strings.SplitN(binVersion, ".", 3)

		majorVersion, err := strconv.ParseUint(splitVersion[0], 10, 64)
		if err != nil {
			log.Fatal(4, "Error parsing git major version: %s", err)
		}
		minorVersion, err := strconv.ParseUint(splitVersion[1], 10, 64)
		if err != nil {
			log.Fatal(4, "Error parsing git minor version: %s", err)
		}
		revisionVersion, err := strconv.ParseUint(splitVersion[2], 10, 64)
		if err != nil {
			log.Fatal(4, "Error parsing git revision version: %s", err)
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

	sec = Cfg.Section("attachment")
	AttachmentPath = sec.Key("PATH").MustString(path.Join(AppDataPath, "attachments"))
	if !filepath.IsAbs(AttachmentPath) {
		AttachmentPath = path.Join(workDir, AttachmentPath)
	}
	AttachmentAllowedTypes = strings.Replace(sec.Key("ALLOWED_TYPES").MustString("image/jpeg,image/png"), "|", ",", -1)
	AttachmentMaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	AttachmentMaxFiles = sec.Key("MAX_FILES").MustInt(5)
	AttachmentEnabled = sec.Key("ENABLE").MustBool(true)

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
	}[Cfg.Section("time").Key("FORMAT").MustString("RFC1123")]

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
	RepoRootPath = sec.Key("ROOT").MustString(path.Join(homeDir, "gitea-repositories"))
	forcePathSeparator(RepoRootPath)
	if !filepath.IsAbs(RepoRootPath) {
		RepoRootPath = path.Join(workDir, RepoRootPath)
	} else {
		RepoRootPath = path.Clean(RepoRootPath)
	}
	ScriptType = sec.Key("SCRIPT_TYPE").MustString("bash")
	if err = Cfg.Section("repository").MapTo(&Repository); err != nil {
		log.Fatal(4, "Fail to map Repository settings: %v", err)
	} else if err = Cfg.Section("repository.editor").MapTo(&Repository.Editor); err != nil {
		log.Fatal(4, "Fail to map Repository.Editor settings: %v", err)
	} else if err = Cfg.Section("repository.upload").MapTo(&Repository.Upload); err != nil {
		log.Fatal(4, "Fail to map Repository.Upload settings: %v", err)
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
		log.Fatal(4, "Fail to map UI settings: %v", err)
	} else if err = Cfg.Section("markdown").MapTo(&Markdown); err != nil {
		log.Fatal(4, "Fail to map Markdown settings: %v", err)
	} else if err = Cfg.Section("cron").MapTo(&Cron); err != nil {
		log.Fatal(4, "Fail to map Cron settings: %v", err)
	} else if err = Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal(4, "Fail to map Git settings: %v", err)
	} else if err = Cfg.Section("mirror").MapTo(&Mirror); err != nil {
		log.Fatal(4, "Fail to map Mirror settings: %v", err)
	} else if err = Cfg.Section("api").MapTo(&API); err != nil {
		log.Fatal(4, "Fail to map API settings: %v", err)
	}

	if Mirror.DefaultInterval <= 0 {
		Mirror.DefaultInterval = 24
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
	log.Info("Gitea v%s", AppVer)

	LogModes = strings.Split(Cfg.Section("log").Key("MODE").MustString("console"), ",")
	LogConfigs = make([]string, len(LogModes))

	for i, mode := range LogModes {
		mode = strings.TrimSpace(mode)

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
			logPath := sec.Key("FILE_NAME").MustString(path.Join(LogRootPath, "gogs.log"))
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
	QueueLength           int
	Name                  string
	From                  string
	FromEmail             string
	EnableHTMLAlternative bool

	// SMTP sender
	Host                  string
	User, Passwd          string
	DisableHelo           bool
	HeloHostname          string
	SkipVerify            bool
	UseCertificate        bool
	CertFile, KeyFile     string

	// Sendmail sender
	UseSendmail           bool
	SendmailPath          string
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
		QueueLength:           sec.Key("SEND_BUFFER_LEN").MustInt(100),
		Name:                  sec.Key("NAME").MustString(AppName),
		EnableHTMLAlternative: sec.Key("ENABLE_HTML_ALTERNATIVE").MustBool(),

		Host:                  sec.Key("HOST").String(),
		User:                  sec.Key("USER").String(),
		Passwd:                sec.Key("PASSWD").String(),
		DisableHelo:           sec.Key("DISABLE_HELO").MustBool(),
		HeloHostname:          sec.Key("HELO_HOSTNAME").String(),
		SkipVerify:            sec.Key("SKIP_VERIFY").MustBool(),
		UseCertificate:        sec.Key("USE_CERTIFICATE").MustBool(),
		CertFile:              sec.Key("CERT_FILE").String(),
		KeyFile:               sec.Key("KEY_FILE").String(),

		UseSendmail:           sec.Key("USE_SENDMAIL").MustBool(),
		SendmailPath:          sec.Key("SENDMAIL_PATH").MustString("sendmail"),
	}
	MailService.From = sec.Key("FROM").MustString(MailService.User)

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
	Webhook.Types = []string{"gogs", "slack"}
	Webhook.PagingNum = sec.Key("PAGING_NUM").MustInt(10)
}

// NewServices initializes the services
func NewServices() {
	newService()
	newLogService()
	newCacheService()
	newSessionService()
	newMailService()
	newRegisterMailService()
	newNotifyMailService()
	newWebhookService()
}
