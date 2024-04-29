// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"math"
	"path/filepath"
	"sync/atomic"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
)

// OAuth2UsernameType is enum describing the way gitea 'name' should be generated from oauth2 data
type OAuth2UsernameType string

const (
	OAuth2UsernameUserid            OAuth2UsernameType = "userid"             // use user id (sub) field as gitea's username
	OAuth2UsernameNickname          OAuth2UsernameType = "nickname"           // use nickname field
	OAuth2UsernameEmail             OAuth2UsernameType = "email"              // use email field
	OAuth2UsernamePreferredUsername OAuth2UsernameType = "preferred_username" // use preferred_username field
)

func (username OAuth2UsernameType) isValid() bool {
	switch username {
	case OAuth2UsernameUserid, OAuth2UsernameNickname, OAuth2UsernameEmail, OAuth2UsernamePreferredUsername:
		return true
	}
	return false
}

// OAuth2AccountLinkingType is enum describing behaviour of linking with existing account
type OAuth2AccountLinkingType string

const (
	// OAuth2AccountLinkingDisabled error will be displayed if account exist
	OAuth2AccountLinkingDisabled OAuth2AccountLinkingType = "disabled"
	// OAuth2AccountLinkingLogin account linking login will be displayed if account exist
	OAuth2AccountLinkingLogin OAuth2AccountLinkingType = "login"
	// OAuth2AccountLinkingAuto account will be automatically linked if account exist
	OAuth2AccountLinkingAuto OAuth2AccountLinkingType = "auto"
)

func (accountLinking OAuth2AccountLinkingType) isValid() bool {
	switch accountLinking {
	case OAuth2AccountLinkingDisabled, OAuth2AccountLinkingLogin, OAuth2AccountLinkingAuto:
		return true
	}
	return false
}

// OAuth2Client settings
var OAuth2Client struct {
	RegisterEmailConfirm   bool
	OpenIDConnectScopes    []string
	EnableAutoRegistration bool
	Username               OAuth2UsernameType
	UpdateAvatar           bool
	AccountLinking         OAuth2AccountLinkingType
}

func loadOAuth2ClientFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("oauth2_client")
	OAuth2Client.RegisterEmailConfirm = sec.Key("REGISTER_EMAIL_CONFIRM").MustBool(Service.RegisterEmailConfirm)
	OAuth2Client.OpenIDConnectScopes = parseScopes(sec, "OPENID_CONNECT_SCOPES")
	OAuth2Client.EnableAutoRegistration = sec.Key("ENABLE_AUTO_REGISTRATION").MustBool()
	OAuth2Client.Username = OAuth2UsernameType(sec.Key("USERNAME").MustString(string(OAuth2UsernameNickname)))
	if !OAuth2Client.Username.isValid() {
		OAuth2Client.Username = OAuth2UsernameNickname
		log.Warn("[oauth2_client].USERNAME setting is invalid, falls back to %q", OAuth2Client.Username)
	}
	OAuth2Client.UpdateAvatar = sec.Key("UPDATE_AVATAR").MustBool()
	OAuth2Client.AccountLinking = OAuth2AccountLinkingType(sec.Key("ACCOUNT_LINKING").MustString(string(OAuth2AccountLinkingLogin)))
	if !OAuth2Client.AccountLinking.isValid() {
		log.Warn("Account linking setting is not valid: '%s', will fallback to '%s'", OAuth2Client.AccountLinking, OAuth2AccountLinkingLogin)
		OAuth2Client.AccountLinking = OAuth2AccountLinkingLogin
	}
}

func parseScopes(sec ConfigSection, name string) []string {
	parts := sec.Key(name).Strings(" ")
	scopes := make([]string, 0, len(parts))
	for _, scope := range parts {
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

var OAuth2 = struct {
	Enabled                    bool
	AccessTokenExpirationTime  int64
	RefreshTokenExpirationTime int64
	InvalidateRefreshTokens    bool
	JWTSigningAlgorithm        string `ini:"JWT_SIGNING_ALGORITHM"`
	JWTSigningPrivateKeyFile   string `ini:"JWT_SIGNING_PRIVATE_KEY_FILE"`
	MaxTokenLength             int
	DefaultApplications        []string
}{
	Enabled:                    true,
	AccessTokenExpirationTime:  3600,
	RefreshTokenExpirationTime: 730,
	InvalidateRefreshTokens:    false,
	JWTSigningAlgorithm:        "RS256",
	JWTSigningPrivateKeyFile:   "jwt/private.pem",
	MaxTokenLength:             math.MaxInt16,
	DefaultApplications:        []string{"git-credential-oauth", "git-credential-manager", "tea"},
}

func loadOAuth2From(rootCfg ConfigProvider) {
	sec := rootCfg.Section("oauth2")
	if err := sec.MapTo(&OAuth2); err != nil {
		log.Fatal("Failed to map OAuth2 settings: %v", err)
		return
	}

	if sec.HasKey("DEFAULT_APPLICATIONS") && sec.Key("DEFAULT_APPLICATIONS").String() == "" {
		OAuth2.DefaultApplications = nil
	}

	// Handle the rename of ENABLE to ENABLED
	deprecatedSetting(rootCfg, "oauth2", "ENABLE", "oauth2", "ENABLED", "v1.23.0")
	if sec.HasKey("ENABLE") && !sec.HasKey("ENABLED") {
		OAuth2.Enabled = sec.Key("ENABLE").MustBool(OAuth2.Enabled)
	}

	if !OAuth2.Enabled {
		return
	}

	jwtSecretBase64 := loadSecret(sec, "JWT_SECRET_URI", "JWT_SECRET")

	if !filepath.IsAbs(OAuth2.JWTSigningPrivateKeyFile) {
		OAuth2.JWTSigningPrivateKeyFile = filepath.Join(AppDataPath, OAuth2.JWTSigningPrivateKeyFile)
	}

	if InstallLock {
		jwtSecretBytes, err := generate.DecodeJwtSecretBase64(jwtSecretBase64)
		if err != nil {
			jwtSecretBytes, jwtSecretBase64, err = generate.NewJwtSecretWithBase64()
			if err != nil {
				log.Fatal("error generating JWT secret: %v", err)
			}
			saveCfg, err := rootCfg.PrepareSaving()
			if err != nil {
				log.Fatal("save oauth2.JWT_SECRET failed: %v", err)
			}
			rootCfg.Section("oauth2").Key("JWT_SECRET").SetValue(jwtSecretBase64)
			saveCfg.Section("oauth2").Key("JWT_SECRET").SetValue(jwtSecretBase64)
			if err := saveCfg.Save(); err != nil {
				log.Fatal("save oauth2.JWT_SECRET failed: %v", err)
			}
		}
		generalSigningSecret.Store(&jwtSecretBytes)
	}
}

// generalSigningSecret is used as container for a []byte value
// instead of an additional mutex, we use CompareAndSwap func to change the value thread save
var generalSigningSecret atomic.Pointer[[]byte]

func GetGeneralTokenSigningSecret() []byte {
	old := generalSigningSecret.Load()
	if old == nil || len(*old) == 0 {
		jwtSecret, _, err := generate.NewJwtSecretWithBase64()
		if err != nil {
			log.Fatal("Unable to generate general JWT secret: %s", err.Error())
		}
		if generalSigningSecret.CompareAndSwap(old, &jwtSecret) {
			// FIXME: in main branch, the signing token should be refactored (eg: one unique for LFS/OAuth2/etc ...)
			LogStartupProblem(1, log.WARN, "OAuth2 is not enabled, unable to use a persistent signing secret, a new one is generated, which is not persistent between restarts and cluster nodes")
			return jwtSecret
		}
		return *generalSigningSecret.Load()
	}
	return *old
}
