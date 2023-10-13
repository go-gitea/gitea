// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"encoding/base64"
	"math"
	"path/filepath"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// OAuth2UsernameType is enum describing the way gitea 'name' should be generated from oauth2 data
type OAuth2UsernameType string

const (
	// OAuth2UsernameUserid oauth2 userid field will be used as gitea name
	OAuth2UsernameUserid OAuth2UsernameType = "userid"
	// OAuth2UsernameNickname oauth2 nickname field will be used as gitea name
	OAuth2UsernameNickname OAuth2UsernameType = "nickname"
	// OAuth2UsernameEmail username of oauth2 email filed will be used as gitea name
	OAuth2UsernameEmail OAuth2UsernameType = "email"
)

func (username OAuth2UsernameType) isValid() bool {
	switch username {
	case OAuth2UsernameUserid, OAuth2UsernameNickname, OAuth2UsernameEmail:
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
		log.Warn("Username setting is not valid: '%s', will fallback to '%s'", OAuth2Client.Username, OAuth2UsernameNickname)
		OAuth2Client.Username = OAuth2UsernameNickname
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
	Enable                     bool
	AccessTokenExpirationTime  int64
	RefreshTokenExpirationTime int64
	InvalidateRefreshTokens    bool
	JWTSigningAlgorithm        string `ini:"JWT_SIGNING_ALGORITHM"`
	JWTSecretBase64            string `ini:"JWT_SECRET"`
	JWTSigningPrivateKeyFile   string `ini:"JWT_SIGNING_PRIVATE_KEY_FILE"`
	MaxTokenLength             int
	DefaultApplications        []string
}{
	Enable:                     true,
	AccessTokenExpirationTime:  3600,
	RefreshTokenExpirationTime: 730,
	InvalidateRefreshTokens:    false,
	JWTSigningAlgorithm:        "RS256",
	JWTSigningPrivateKeyFile:   "jwt/private.pem",
	MaxTokenLength:             math.MaxInt16,
	DefaultApplications:        []string{"git-credential-oauth", "git-credential-manager", "tea"},
}

func loadOAuth2From(rootCfg ConfigProvider) {
	if err := rootCfg.Section("oauth2").MapTo(&OAuth2); err != nil {
		log.Fatal("Failed to OAuth2 settings: %v", err)
		return
	}

	if !OAuth2.Enable {
		return
	}

	OAuth2.JWTSecretBase64 = loadSecret(rootCfg.Section("oauth2"), "JWT_SECRET_URI", "JWT_SECRET")

	if !filepath.IsAbs(OAuth2.JWTSigningPrivateKeyFile) {
		OAuth2.JWTSigningPrivateKeyFile = filepath.Join(AppDataPath, OAuth2.JWTSigningPrivateKeyFile)
	}

	if InstallLock {
		if _, err := util.Base64FixedDecode(base64.RawURLEncoding, []byte(OAuth2.JWTSecretBase64), 32); err != nil {
			key, err := generate.NewJwtSecret()
			if err != nil {
				log.Fatal("error generating JWT secret: %v", err)
			}

			OAuth2.JWTSecretBase64 = base64.RawURLEncoding.EncodeToString(key)
			saveCfg, err := rootCfg.PrepareSaving()
			if err != nil {
				log.Fatal("save oauth2.JWT_SECRET failed: %v", err)
			}
			rootCfg.Section("oauth2").Key("JWT_SECRET").SetValue(OAuth2.JWTSecretBase64)
			saveCfg.Section("oauth2").Key("JWT_SECRET").SetValue(OAuth2.JWTSecretBase64)
			if err := saveCfg.Save(); err != nil {
				log.Fatal("save oauth2.JWT_SECRET failed: %v", err)
			}
		}
	}
}
