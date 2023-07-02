// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"encoding/base64"
	"net"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
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

var (
	// AppName is the Application name, used in the page title.
	// It maps to ini:"APP_NAME"
	AppName string
	// AppURL is the Application ROOT_URL. It always has a '/' suffix
	// It maps to ini:"ROOT_URL"
	AppURL string
	// AppSubURL represents the sub-url mounting point for gitea. It is either "" or starts with '/' and ends without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL string
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

	HasRobotsTxt bool
	ManifestData string
)

// MakeManifestData generates web app manifest JSON
func MakeManifestData(appName, appURL, absoluteAssetURL string) []byte {
	type manifestIcon struct {
		Src   string `json:"src"`
		Type  string `json:"type"`
		Sizes string `json:"sizes"`
	}

	type manifestJSON struct {
		Name      string         `json:"name"`
		ShortName string         `json:"short_name"`
		StartURL  string         `json:"start_url"`
		Icons     []manifestIcon `json:"icons"`
	}

	bytes, err := json.Marshal(&manifestJSON{
		Name:      appName,
		ShortName: appName,
		StartURL:  appURL,
		Icons: []manifestIcon{
			{
				Src:   absoluteAssetURL + "/assets/img/logo.png",
				Type:  "image/png",
				Sizes: "512x512",
			},
			{
				Src:   absoluteAssetURL + "/assets/img/logo.svg",
				Type:  "image/svg+xml",
				Sizes: "512x512",
			},
		},
	})
	if err != nil {
		log.Error("unable to marshal manifest JSON. Error: %v", err)
		return make([]byte, 0)
	}

	return bytes
}

// MakeAbsoluteAssetURL returns the absolute asset url prefix without a trailing slash
func MakeAbsoluteAssetURL(appURL, staticURLPrefix string) string {
	parsedPrefix, err := url.Parse(strings.TrimSuffix(staticURLPrefix, "/"))
	if err != nil {
		log.Fatal("Unable to parse STATIC_URL_PREFIX: %v", err)
	}

	if err == nil && parsedPrefix.Hostname() == "" {
		if staticURLPrefix == "" {
			return strings.TrimSuffix(appURL, "/")
		}

		// StaticURLPrefix is just a path
		return util.URLJoin(appURL, strings.TrimSuffix(staticURLPrefix, "/"))
	}

	return strings.TrimSuffix(staticURLPrefix, "/")
}

func loadServerFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("server")
	AppName = rootCfg.Section("").Key("APP_NAME").MustString("Gitea: Git with a cup of tea")

	Domain = sec.Key("DOMAIN").MustString("localhost")
	HTTPAddr = sec.Key("HTTP_ADDR").MustString("0.0.0.0")
	HTTPPort = sec.Key("HTTP_PORT").MustString("3000")

	Protocol = HTTP
	protocolCfg := sec.Key("PROTOCOL").String()
	switch protocolCfg {
	case "https":
		Protocol = HTTPS

		// DEPRECATED should not be removed because users maybe upgrade from lower version to the latest version
		// if these are removed, the warning will not be shown
		if sec.HasKey("ENABLE_ACME") {
			EnableAcme = sec.Key("ENABLE_ACME").MustBool(false)
		} else {
			deprecatedSetting(rootCfg, "server", "ENABLE_LETSENCRYPT", "server", "ENABLE_ACME", "v1.19.0")
			EnableAcme = sec.Key("ENABLE_LETSENCRYPT").MustBool(false)
		}
		if EnableAcme {
			AcmeURL = sec.Key("ACME_URL").MustString("")
			AcmeCARoot = sec.Key("ACME_CA_ROOT").MustString("")

			if sec.HasKey("ACME_ACCEPTTOS") {
				AcmeTOS = sec.Key("ACME_ACCEPTTOS").MustBool(false)
			} else {
				deprecatedSetting(rootCfg, "server", "LETSENCRYPT_ACCEPTTOS", "server", "ACME_ACCEPTTOS", "v1.19.0")
				AcmeTOS = sec.Key("LETSENCRYPT_ACCEPTTOS").MustBool(false)
			}
			if !AcmeTOS {
				log.Fatal("ACME TOS is not accepted (ACME_ACCEPTTOS).")
			}

			if sec.HasKey("ACME_DIRECTORY") {
				AcmeLiveDirectory = sec.Key("ACME_DIRECTORY").MustString("https")
			} else {
				deprecatedSetting(rootCfg, "server", "LETSENCRYPT_DIRECTORY", "server", "ACME_DIRECTORY", "v1.19.0")
				AcmeLiveDirectory = sec.Key("LETSENCRYPT_DIRECTORY").MustString("https")
			}

			if sec.HasKey("ACME_EMAIL") {
				AcmeEmail = sec.Key("ACME_EMAIL").MustString("")
			} else {
				deprecatedSetting(rootCfg, "server", "LETSENCRYPT_EMAIL", "server", "ACME_EMAIL", "v1.19.0")
				AcmeEmail = sec.Key("LETSENCRYPT_EMAIL").MustString("")
			}
		} else {
			CertFile = sec.Key("CERT_FILE").String()
			KeyFile = sec.Key("KEY_FILE").String()
			if len(CertFile) > 0 && !filepath.IsAbs(CertFile) {
				CertFile = filepath.Join(CustomPath, CertFile)
			}
			if len(KeyFile) > 0 && !filepath.IsAbs(KeyFile) {
				KeyFile = filepath.Join(CustomPath, KeyFile)
			}
		}
		SSLMinimumVersion = sec.Key("SSL_MIN_VERSION").MustString("")
		SSLMaximumVersion = sec.Key("SSL_MAX_VERSION").MustString("")
		SSLCurvePreferences = sec.Key("SSL_CURVE_PREFERENCES").Strings(",")
		SSLCipherSuites = sec.Key("SSL_CIPHER_SUITES").Strings(",")
	case "fcgi":
		Protocol = FCGI
	case "fcgi+unix", "unix", "http+unix":
		switch protocolCfg {
		case "fcgi+unix":
			Protocol = FCGIUnix
		case "unix":
			log.Warn("unix PROTOCOL value is deprecated, please use http+unix")
			fallthrough
		case "http+unix":
			Protocol = HTTPUnix
		}
		UnixSocketPermissionRaw := sec.Key("UNIX_SOCKET_PERMISSION").MustString("666")
		UnixSocketPermissionParsed, err := strconv.ParseUint(UnixSocketPermissionRaw, 8, 32)
		if err != nil || UnixSocketPermissionParsed > 0o777 {
			log.Fatal("Failed to parse unixSocketPermission: %s", UnixSocketPermissionRaw)
		}

		UnixSocketPermission = uint32(UnixSocketPermissionParsed)
		if !filepath.IsAbs(HTTPAddr) {
			HTTPAddr = filepath.Join(AppWorkPath, HTTPAddr)
		}
	}
	UseProxyProtocol = sec.Key("USE_PROXY_PROTOCOL").MustBool(false)
	ProxyProtocolTLSBridging = sec.Key("PROXY_PROTOCOL_TLS_BRIDGING").MustBool(false)
	ProxyProtocolHeaderTimeout = sec.Key("PROXY_PROTOCOL_HEADER_TIMEOUT").MustDuration(5 * time.Second)
	ProxyProtocolAcceptUnknown = sec.Key("PROXY_PROTOCOL_ACCEPT_UNKNOWN").MustBool(false)
	GracefulRestartable = sec.Key("ALLOW_GRACEFUL_RESTARTS").MustBool(true)
	GracefulHammerTime = sec.Key("GRACEFUL_HAMMER_TIME").MustDuration(60 * time.Second)
	StartupTimeout = sec.Key("STARTUP_TIMEOUT").MustDuration(0 * time.Second)
	PerWriteTimeout = sec.Key("PER_WRITE_TIMEOUT").MustDuration(PerWriteTimeout)
	PerWritePerKbTimeout = sec.Key("PER_WRITE_PER_KB_TIMEOUT").MustDuration(PerWritePerKbTimeout)

	defaultAppURL := string(Protocol) + "://" + Domain + ":" + HTTPPort
	AppURL = sec.Key("ROOT_URL").MustString(defaultAppURL)

	// Check validity of AppURL
	appURL, err := url.Parse(AppURL)
	if err != nil {
		log.Fatal("Invalid ROOT_URL '%s': %s", AppURL, err)
	}
	// Remove default ports from AppURL.
	// (scheme-based URL normalization, RFC 3986 section 6.2.3)
	if (appURL.Scheme == string(HTTP) && appURL.Port() == "80") || (appURL.Scheme == string(HTTPS) && appURL.Port() == "443") {
		appURL.Host = appURL.Hostname()
	}
	// This should be TrimRight to ensure that there is only a single '/' at the end of AppURL.
	AppURL = strings.TrimRight(appURL.String(), "/") + "/"

	// Suburl should start with '/' and end without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL = strings.TrimSuffix(appURL.Path, "/")
	StaticURLPrefix = strings.TrimSuffix(sec.Key("STATIC_URL_PREFIX").MustString(AppSubURL), "/")

	// Check if Domain differs from AppURL domain than update it to AppURL's domain
	urlHostname := appURL.Hostname()
	if urlHostname != Domain && net.ParseIP(urlHostname) == nil && urlHostname != "" {
		Domain = urlHostname
	}

	AbsoluteAssetURL = MakeAbsoluteAssetURL(AppURL, StaticURLPrefix)
	AssetVersion = strings.ReplaceAll(AppVer, "+", "~") // make sure the version string is clear (no real escaping is needed)

	manifestBytes := MakeManifestData(AppName, AppURL, AbsoluteAssetURL)
	ManifestData = `application/json;base64,` + base64.StdEncoding.EncodeToString(manifestBytes)

	var defaultLocalURL string
	switch Protocol {
	case HTTPUnix:
		defaultLocalURL = "http://unix/"
	case FCGI:
		defaultLocalURL = AppURL
	case FCGIUnix:
		defaultLocalURL = AppURL
	default:
		defaultLocalURL = string(Protocol) + "://"
		if HTTPAddr == "0.0.0.0" {
			defaultLocalURL += net.JoinHostPort("localhost", HTTPPort) + "/"
		} else {
			defaultLocalURL += net.JoinHostPort(HTTPAddr, HTTPPort) + "/"
		}
	}
	LocalURL = sec.Key("LOCAL_ROOT_URL").MustString(defaultLocalURL)
	LocalURL = strings.TrimRight(LocalURL, "/") + "/"
	LocalUseProxyProtocol = sec.Key("LOCAL_USE_PROXY_PROTOCOL").MustBool(UseProxyProtocol)
	RedirectOtherPort = sec.Key("REDIRECT_OTHER_PORT").MustBool(false)
	PortToRedirect = sec.Key("PORT_TO_REDIRECT").MustString("80")
	RedirectorUseProxyProtocol = sec.Key("REDIRECTOR_USE_PROXY_PROTOCOL").MustBool(UseProxyProtocol)
	OfflineMode = sec.Key("OFFLINE_MODE").MustBool()
	if len(StaticRootPath) == 0 {
		StaticRootPath = AppWorkPath
	}
	StaticRootPath = sec.Key("STATIC_ROOT_PATH").MustString(StaticRootPath)
	StaticCacheTime = sec.Key("STATIC_CACHE_TIME").MustDuration(6 * time.Hour)
	AppDataPath = sec.Key("APP_DATA_PATH").MustString(path.Join(AppWorkPath, "data"))
	if !filepath.IsAbs(AppDataPath) {
		AppDataPath = filepath.ToSlash(filepath.Join(AppWorkPath, AppDataPath))
	}

	EnableGzip = sec.Key("ENABLE_GZIP").MustBool()
	EnablePprof = sec.Key("ENABLE_PPROF").MustBool(false)
	PprofDataPath = sec.Key("PPROF_DATA_PATH").MustString(path.Join(AppWorkPath, "data/tmp/pprof"))
	if !filepath.IsAbs(PprofDataPath) {
		PprofDataPath = filepath.Join(AppWorkPath, PprofDataPath)
	}

	landingPage := sec.Key("LANDING_PAGE").MustString("home")
	switch landingPage {
	case "explore":
		LandingPageURL = LandingPageExplore
	case "organizations":
		LandingPageURL = LandingPageOrganizations
	case "login":
		LandingPageURL = LandingPageLogin
	case "":
	case "home":
		LandingPageURL = LandingPageHome
	default:
		LandingPageURL = LandingPage(landingPage)
	}

	HasRobotsTxt, err = util.IsFile(path.Join(CustomPath, "robots.txt"))
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", path.Join(CustomPath, "robots.txt"), err)
	}
}
