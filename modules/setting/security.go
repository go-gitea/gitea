// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
)

var (
	// Security settings
	InstallLock                        bool
	SecretKey                          string
	InternalToken                      string // internal access token
	LogInRememberDays                  int
	CookieRememberName                 string
	ReverseProxyAuthUser               string
	ReverseProxyAuthEmail              string
	ReverseProxyAuthFullName           string
	ReverseProxyLimit                  int
	ReverseProxyTrustedProxies         []string
	MinPasswordLength                  int
	ImportLocalPaths                   bool
	DisableGitHooks                    bool
	DisableWebhooks                    bool
	OnlyAllowPushIfGiteaEnvironmentSet bool
	PasswordComplexity                 []string
	PasswordHashAlgo                   string
	PasswordCheckPwn                   bool
	SuccessfulTokensCacheSize          int
	CSRFCookieName                     = "_csrf"
	CSRFCookieHTTPOnly                 = true
)

// loadSecret load the secret from ini by uriKey or verbatimKey, only one of them could be set
// If the secret is loaded from uriKey (file), the file should be non-empty, to guarantee the behavior stable and clear.
func loadSecret(sec ConfigSection, uriKey, verbatimKey string) string {
	// don't allow setting both URI and verbatim string
	uri := sec.Key(uriKey).String()
	verbatim := sec.Key(verbatimKey).String()
	if uri != "" && verbatim != "" {
		log.Fatal("Cannot specify both %s and %s", uriKey, verbatimKey)
	}

	// if we have no URI, use verbatim
	if uri == "" {
		return verbatim
	}

	tempURI, err := url.Parse(uri)
	if err != nil {
		log.Fatal("Failed to parse %s (%s): %v", uriKey, uri, err)
	}
	switch tempURI.Scheme {
	case "file":
		buf, err := os.ReadFile(tempURI.RequestURI())
		if err != nil {
			log.Fatal("Failed to read %s (%s): %v", uriKey, tempURI.RequestURI(), err)
		}
		val := strings.TrimSpace(string(buf))
		if val == "" {
			// The file shouldn't be empty, otherwise we can not know whether the user has ever set the KEY or KEY_URI
			// For example: if INTERNAL_TOKEN_URI=file:///empty-file,
			// Then if the token is re-generated during installation and saved to INTERNAL_TOKEN
			// Then INTERNAL_TOKEN and INTERNAL_TOKEN_URI both exist, that's a fatal error (they shouldn't)
			log.Fatal("Failed to read %s (%s): the file is empty", uriKey, tempURI.RequestURI())
		}
		return val

	// only file URIs are allowed
	default:
		log.Fatal("Unsupported URI-Scheme %q (%q = %q)", tempURI.Scheme, uriKey, uri)
		return ""
	}
}

// generateSaveInternalToken generates and saves the internal token to app.ini
func generateSaveInternalToken(rootCfg ConfigProvider) {
	token, err := generate.NewInternalToken()
	if err != nil {
		log.Fatal("Error generate internal token: %v", err)
	}

	InternalToken = token
	saveCfg, err := rootCfg.PrepareSaving()
	if err != nil {
		log.Fatal("Error saving internal token: %v", err)
	}
	rootCfg.Section("security").Key("INTERNAL_TOKEN").SetValue(token)
	saveCfg.Section("security").Key("INTERNAL_TOKEN").SetValue(token)
	if err = saveCfg.Save(); err != nil {
		log.Fatal("Error saving internal token: %v", err)
	}
}

func loadSecurityFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("security")
	InstallLock = HasInstallLock(rootCfg)
	LogInRememberDays = sec.Key("LOGIN_REMEMBER_DAYS").MustInt(7)
	SecretKey = loadSecret(sec, "SECRET_KEY_URI", "SECRET_KEY")
	if SecretKey == "" {
		// FIXME: https://github.com/go-gitea/gitea/issues/16832
		// Until it supports rotating an existing secret key, we shouldn't move users off of the widely used default value
		SecretKey = "!#@FDEWREWR&*(" //nolint:gosec
	}

	CookieRememberName = sec.Key("COOKIE_REMEMBER_NAME").MustString("gitea_incredible")

	ReverseProxyAuthUser = sec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")
	ReverseProxyAuthEmail = sec.Key("REVERSE_PROXY_AUTHENTICATION_EMAIL").MustString("X-WEBAUTH-EMAIL")
	ReverseProxyAuthFullName = sec.Key("REVERSE_PROXY_AUTHENTICATION_FULL_NAME").MustString("X-WEBAUTH-FULLNAME")

	ReverseProxyLimit = sec.Key("REVERSE_PROXY_LIMIT").MustInt(1)
	ReverseProxyTrustedProxies = sec.Key("REVERSE_PROXY_TRUSTED_PROXIES").Strings(",")
	if len(ReverseProxyTrustedProxies) == 0 {
		ReverseProxyTrustedProxies = []string{"127.0.0.0/8", "::1/128"}
	}

	MinPasswordLength = sec.Key("MIN_PASSWORD_LENGTH").MustInt(8)
	ImportLocalPaths = sec.Key("IMPORT_LOCAL_PATHS").MustBool(false)
	DisableGitHooks = sec.Key("DISABLE_GIT_HOOKS").MustBool(true)
	DisableWebhooks = sec.Key("DISABLE_WEBHOOKS").MustBool(false)
	OnlyAllowPushIfGiteaEnvironmentSet = sec.Key("ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET").MustBool(true)

	// Ensure that the provided default hash algorithm is a valid hash algorithm
	var algorithm *hash.PasswordHashAlgorithm
	PasswordHashAlgo, algorithm = hash.SetDefaultPasswordHashAlgorithm(sec.Key("PASSWORD_HASH_ALGO").MustString(""))
	if algorithm == nil {
		log.Fatal("The provided password hash algorithm was invalid: %s", sec.Key("PASSWORD_HASH_ALGO").MustString(""))
	}

	CSRFCookieHTTPOnly = sec.Key("CSRF_COOKIE_HTTP_ONLY").MustBool(true)
	PasswordCheckPwn = sec.Key("PASSWORD_CHECK_PWN").MustBool(false)
	SuccessfulTokensCacheSize = sec.Key("SUCCESSFUL_TOKENS_CACHE_SIZE").MustInt(20)

	InternalToken = loadSecret(sec, "INTERNAL_TOKEN_URI", "INTERNAL_TOKEN")
	if InstallLock && InternalToken == "" {
		// if Gitea has been installed but the InternalToken hasn't been generated (upgrade from an old release), we should generate
		// some users do cluster deployment, they still depend on this auto-generating behavior.
		generateSaveInternalToken(rootCfg)
	}

	cfgdata := sec.Key("PASSWORD_COMPLEXITY").Strings(",")
	if len(cfgdata) == 0 {
		cfgdata = []string{"off"}
	}
	PasswordComplexity = make([]string, 0, len(cfgdata))
	for _, name := range cfgdata {
		name := strings.ToLower(strings.Trim(name, `"`))
		if name != "" {
			PasswordComplexity = append(PasswordComplexity, name)
		}
	}
}
