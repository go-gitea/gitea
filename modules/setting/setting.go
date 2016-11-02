// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Unknwon/com"
	_ "github.com/go-macaron/cache/memcache"
	_ "github.com/go-macaron/cache/redis"
	"github.com/go-macaron/session"
	_ "github.com/go-macaron/session/redis"
	"strk.kbt.io/projects/go/libravatar"
	"gopkg.in/ini.v1"

	"github.com/gogits/gogs/modules/bindata"
	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/user"
)

type Scheme string

const (
	HTTP        Scheme = "http"
	HTTPS       Scheme = "https"
	FCGI        Scheme = "fcgi"
	UNIX_SOCKET Scheme = "unix"
)

type LandingPage string

const (
	LANDING_PAGE_HOME    LandingPage = "/"
	LANDING_PAGE_EXPLORE LandingPage = "/explore"
)

var (
	// Build information should only be set by -ldflags.
	BuildTime    string
	BuildGitHash string

	// App settings
	AppVer         string
	AppName        string
	AppUrl         string
	AppSubUrl      string
	AppSubUrlDepth int // Number of slashes
	AppPath        string
	AppDataPath    string

	// Server settings
	Protocol             Scheme
	Domain               string
	HTTPAddr, HTTPPort   string
	LocalURL             string
	OfflineMode          bool
	DisableRouterLog     bool
	CertFile, KeyFile    string
	StaticRootPath       string
	EnableGzip           bool
	LandingPageURL       LandingPage
	UnixSocketPermission uint32

	SSH struct {
		Disabled            bool           `ini:"DISABLE_SSH"`
		StartBuiltinServer  bool           `ini:"START_SSH_SERVER"`
		Domain              string         `ini:"SSH_DOMAIN"`
		Port                int            `ini:"SSH_PORT"`
		ListenPort          int            `ini:"SSH_LISTEN_PORT"`
		RootPath            string         `ini:"SSH_ROOT_PATH"`
		KeyTestPath         string         `ini:"SSH_KEY_TEST_PATH"`
		KeygenPath          string         `ini:"SSH_KEYGEN_PATH"`
		MinimumKeySizeCheck bool           `ini:"-"`
		MinimumKeySizes     map[string]int `ini:"-"`
	}

	// Security settings
	InstallLock          bool
	SecretKey            string
	LogInRememberDays    int
	CookieUserName       string
	CookieRememberName   string
	ReverseProxyAuthUser string

	// Database settings
	UseSQLite3    bool
	UseMySQL      bool
	UsePostgreSQL bool
	UseTiDB       bool

	// Webhook settings
	Webhook struct {
		QueueLength    int
		DeliverTimeout int
		SkipTLSVerify  bool
		Types          []string
		PagingNum      int
	}

	// Repository settings
	Repository struct {
		AnsiCharset            string
		ForcePrivate           bool
		MaxCreationLimit       int
		MirrorQueueLength      int
		PullRequestQueueLength int
		PreferredLicenses      []string

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
	}
	RepoRootPath string
	ScriptType   string

	// UI settings
	UI struct {
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
	}

	// Markdown sttings
	Markdown struct {
		EnableHardLineBreak bool
		CustomURLSchemes    []string `ini:"CUSTOM_URL_SCHEMES"`
		FileExtensions      []string
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
	Cron struct {
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
	}

	// Git settings
	Git struct {
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
	}

	// Mirror settings
	Mirror struct {
		DefaultInterval int
	}

	// API settings
	API struct {
		MaxResponseItems int
	}

	// I18n settings
	Langs, Names []string
	dateLangs    map[string]string

	// Highlight settings are loaded in modules/template/hightlight.go

	// Other settings
	ShowFooterBranding         bool
	ShowFooterVersion          bool
	ShowFooterTemplateLoadTime bool
	SupportMiniWinService      bool

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
	wd := os.Getenv("GOGS_WORK_DIR")
	if len(wd) > 0 {
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

	Cfg, err = ini.Load(bindata.MustAsset("conf/app.ini"))
	if err != nil {
		log.Fatal(4, "Fail to parse 'conf/app.ini': %v", err)
	}

	CustomPath = os.Getenv("GOGS_CUSTOM")
	if len(CustomPath) == 0 {
		CustomPath = workDir + "/custom"
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
	AppName = Cfg.Section("").Key("APP_NAME").MustString("Gogs: Go Git Service")
	AppUrl = sec.Key("ROOT_URL").MustString("http://localhost:3000/")
	if AppUrl[len(AppUrl)-1] != '/' {
		AppUrl += "/"
	}

	// Check if has app suburl.
	url, err := url.Parse(AppUrl)
	if err != nil {
		log.Fatal(4, "Invalid ROOT_URL '%s': %s", AppUrl, err)
	}
	// Suburl should start with '/' and end without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubUrl = strings.TrimSuffix(url.Path, "/")
	AppSubUrlDepth = strings.Count(AppSubUrl, "/")

	Protocol = HTTP
	if sec.Key("PROTOCOL").String() == "https" {
		Protocol = HTTPS
		CertFile = sec.Key("CERT_FILE").String()
		KeyFile = sec.Key("KEY_FILE").String()
	} else if sec.Key("PROTOCOL").String() == "fcgi" {
		Protocol = FCGI
	} else if sec.Key("PROTOCOL").String() == "unix" {
		Protocol = UNIX_SOCKET
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
		LandingPageURL = LANDING_PAGE_EXPLORE
	default:
		LandingPageURL = LANDING_PAGE_HOME
	}

	SSH.RootPath = path.Join(homeDir, ".ssh")
	SSH.KeyTestPath = os.TempDir()
	if err = Cfg.Section("server").MapTo(&SSH); err != nil {
		log.Fatal(4, "Fail to map SSH settings: %v", err)
	}
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

	sec = Cfg.Section("security")
	InstallLock = sec.Key("INSTALL_LOCK").MustBool()
	SecretKey = sec.Key("SECRET_KEY").String()
	LogInRememberDays = sec.Key("LOGIN_REMEMBER_DAYS").MustInt()
	CookieUserName = sec.Key("COOKIE_USERNAME").String()
	CookieRememberName = sec.Key("COOKIE_REMEMBER_NAME").String()
	ReverseProxyAuthUser = sec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")

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

	RunUser = Cfg.Section("").Key("RUN_USER").String()
	// Does not check run user when the install lock is off.
	if InstallLock {
		currentUser, match := IsRunUserMatchCurrentUser(RunUser)
		if !match {
			log.Fatal(4, "Expect user '%s' but current user is: %s", RunUser, currentUser)
		}
	}

	// Determine and create root git repository path.
	sec = Cfg.Section("repository")
	RepoRootPath = sec.Key("ROOT").MustString(path.Join(homeDir, "gogs-repositories"))
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
	Names = Cfg.Section("i18n").Key("NAMES").Strings(",")
	dateLangs = Cfg.Section("i18n.datelang").KeysHash()

	ShowFooterBranding = Cfg.Section("other").Key("SHOW_FOOTER_BRANDING").MustBool()
	ShowFooterVersion = Cfg.Section("other").Key("SHOW_FOOTER_VERSION").MustBool()
	ShowFooterTemplateLoadTime = Cfg.Section("other").Key("SHOW_FOOTER_TEMPLATE_LOAD_TIME").MustBool()

	HasRobotsTxt = com.IsFile(path.Join(CustomPath, "robots.txt"))
}

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
	log.Info("%s %s", AppName, AppVer)

	if len(BuildTime) > 0 {
		log.Info("Build Time: %s", BuildTime)
		log.Info("Build Git Hash: %s", BuildGitHash)
	}

	// Get and check log mode.
	LogModes = strings.Split(Cfg.Section("log").Key("MODE").MustString("console"), ",")
	LogConfigs = make([]string, len(LogModes))
	for i, mode := range LogModes {
		mode = strings.TrimSpace(mode)
		sec, err := Cfg.GetSection("log." + mode)
		if err != nil {
			log.Fatal(4, "Unknown log mode: %s", mode)
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
			LogConfigs[i] = fmt.Sprintf(`{"level":%s,"username":"%s","password":"%s","host":"%s","sendTos":"%s","subject":"%s"}`, level,
				sec.Key("USER").MustString("example@example.com"),
				sec.Key("PASSWD").MustString("******"),
				sec.Key("HOST").MustString("127.0.0.1:25"),
				sec.Key("RECEIVERS").MustString("[]"),
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
	SessionConfig.CookieName = Cfg.Section("session").Key("COOKIE_NAME").MustString("i_like_gogits")
	SessionConfig.CookiePath = AppSubUrl
	SessionConfig.Secure = Cfg.Section("session").Key("COOKIE_SECURE").MustBool()
	SessionConfig.Gclifetime = Cfg.Section("session").Key("GC_INTERVAL_TIME").MustInt64(86400)
	SessionConfig.Maxlifetime = Cfg.Section("session").Key("SESSION_LIFE_TIME").MustInt64(86400)

	log.Info("Session Service Enabled")
}

// Mailer represents mail service.
type Mailer struct {
	QueueLength           int
	Name                  string
	Host                  string
	From                  string
	User, Passwd          string
	DisableHelo           bool
	HeloHostname          string
	SkipVerify            bool
	UseCertificate        bool
	CertFile, KeyFile     string
	EnableHTMLAlternative bool
}

var (
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
		Host:                  sec.Key("HOST").String(),
		User:                  sec.Key("USER").String(),
		Passwd:                sec.Key("PASSWD").String(),
		DisableHelo:           sec.Key("DISABLE_HELO").MustBool(),
		HeloHostname:          sec.Key("HELO_HOSTNAME").String(),
		SkipVerify:            sec.Key("SKIP_VERIFY").MustBool(),
		UseCertificate:        sec.Key("USE_CERTIFICATE").MustBool(),
		CertFile:              sec.Key("CERT_FILE").String(),
		KeyFile:               sec.Key("KEY_FILE").String(),
		EnableHTMLAlternative: sec.Key("ENABLE_HTML_ALTERNATIVE").MustBool(),
	}
	MailService.From = sec.Key("FROM").MustString(MailService.User)
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
