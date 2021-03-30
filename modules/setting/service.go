// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"gopkg.in/ini.v1"
	"regexp"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

// Service settings
var Service struct {
	DefaultOrgVisibility                    string
	DefaultOrgVisibilityMode                structs.VisibleType
	ActiveCodeLives                         int
	ResetPwdCodeLives                       int
	RegisterEmailConfirm                    bool
	RegisterManualConfirm                   bool
	EmailDomainWhitelist                    []string
	EmailDomainBlocklist                    []string
	DisableRegistration                     bool
	AllowOnlyExternalRegistration           bool
	ShowRegistrationButton                  bool
	ShowMilestonesDashboardPage             bool
	RequireSignInView                       bool
	EnableNotifyMail                        bool
	EnableBasicAuth                         bool
	EnableReverseProxyAuth                  bool
	EnableReverseProxyAutoRegister          bool
	EnableReverseProxyEmail                 bool
	EnableCaptcha                           bool
	RequireExternalRegistrationCaptcha      bool
	RequireExternalRegistrationPassword     bool
	CaptchaType                             string
	RecaptchaSecret                         string
	RecaptchaSitekey                        string
	RecaptchaURL                            string
	HcaptchaSecret                          string
	HcaptchaSitekey                         string
	DefaultKeepEmailPrivate                 bool
	DefaultAllowCreateOrganization          bool
	EnableTimetracking                      bool
	DefaultEnableTimetracking               bool
	DefaultEnableDependencies               bool
	AllowCrossRepositoryDependencies        bool
	DefaultAllowOnlyContributorsToTrackTime bool
	NoReplyAddress                          string
	EnableUserHeatmap                       bool
	AutoWatchNewRepos                       bool
	AutoWatchOnChanges                      bool
	DefaultOrgMemberVisible                 bool
	UserDeleteWithCommentsMaxTime           time.Duration

	// OpenID settings
	EnableOpenIDSignIn bool
	EnableOpenIDSignUp bool
	OpenIDWhitelist    []*regexp.Regexp
	OpenIDBlacklist    []*regexp.Regexp

	// Explore page settings
	Explore struct {
		RequireSigninView bool `ini:"REQUIRE_SIGNIN_VIEW"`
		DisableUsersPage  bool `ini:"DISABLE_USERS_PAGE"`
	} `ini:"service.explore"`
}

// OAuth2Client settings
var OAuth2Client struct {
	RegisterEmailConfirm   bool
	OpenIDConnectScopes    []string
	GoogleScopes           []string
	EnableAutoRegistration bool
	Username               string
	UpdateAvatar           bool
	AccountLinking         string
}

func newService() {
	sec := Cfg.Section("service")
	Service.ActiveCodeLives = sec.Key("ACTIVE_CODE_LIVE_MINUTES").MustInt(180)
	Service.ResetPwdCodeLives = sec.Key("RESET_PASSWD_CODE_LIVE_MINUTES").MustInt(180)
	Service.DisableRegistration = sec.Key("DISABLE_REGISTRATION").MustBool()
	Service.AllowOnlyExternalRegistration = sec.Key("ALLOW_ONLY_EXTERNAL_REGISTRATION").MustBool()
	if !sec.Key("REGISTER_EMAIL_CONFIRM").MustBool() {
		Service.RegisterManualConfirm = sec.Key("REGISTER_MANUAL_CONFIRM").MustBool(false)
	} else {
		Service.RegisterManualConfirm = false
	}
	Service.EmailDomainWhitelist = sec.Key("EMAIL_DOMAIN_WHITELIST").Strings(",")
	Service.EmailDomainBlocklist = sec.Key("EMAIL_DOMAIN_BLOCKLIST").Strings(",")
	Service.ShowRegistrationButton = sec.Key("SHOW_REGISTRATION_BUTTON").MustBool(!(Service.DisableRegistration || Service.AllowOnlyExternalRegistration))
	Service.ShowMilestonesDashboardPage = sec.Key("SHOW_MILESTONES_DASHBOARD_PAGE").MustBool(true)
	Service.RequireSignInView = sec.Key("REQUIRE_SIGNIN_VIEW").MustBool()
	Service.EnableBasicAuth = sec.Key("ENABLE_BASIC_AUTHENTICATION").MustBool(true)
	Service.EnableReverseProxyAuth = sec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	Service.EnableReverseProxyAutoRegister = sec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	Service.EnableReverseProxyEmail = sec.Key("ENABLE_REVERSE_PROXY_EMAIL").MustBool()
	Service.EnableCaptcha = sec.Key("ENABLE_CAPTCHA").MustBool(false)
	Service.RequireExternalRegistrationCaptcha = sec.Key("REQUIRE_EXTERNAL_REGISTRATION_CAPTCHA").MustBool(Service.EnableCaptcha)
	Service.RequireExternalRegistrationPassword = sec.Key("REQUIRE_EXTERNAL_REGISTRATION_PASSWORD").MustBool()
	Service.CaptchaType = sec.Key("CAPTCHA_TYPE").MustString(ImageCaptcha)
	Service.RecaptchaSecret = sec.Key("RECAPTCHA_SECRET").MustString("")
	Service.RecaptchaSitekey = sec.Key("RECAPTCHA_SITEKEY").MustString("")
	Service.RecaptchaURL = sec.Key("RECAPTCHA_URL").MustString("https://www.google.com/recaptcha/")
	Service.HcaptchaSecret = sec.Key("HCAPTCHA_SECRET").MustString("")
	Service.HcaptchaSitekey = sec.Key("HCAPTCHA_SITEKEY").MustString("")
	Service.DefaultKeepEmailPrivate = sec.Key("DEFAULT_KEEP_EMAIL_PRIVATE").MustBool()
	Service.DefaultAllowCreateOrganization = sec.Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").MustBool(true)
	Service.EnableTimetracking = sec.Key("ENABLE_TIMETRACKING").MustBool(true)
	if Service.EnableTimetracking {
		Service.DefaultEnableTimetracking = sec.Key("DEFAULT_ENABLE_TIMETRACKING").MustBool(true)
	}
	Service.DefaultEnableDependencies = sec.Key("DEFAULT_ENABLE_DEPENDENCIES").MustBool(true)
	Service.AllowCrossRepositoryDependencies = sec.Key("ALLOW_CROSS_REPOSITORY_DEPENDENCIES").MustBool(true)
	Service.DefaultAllowOnlyContributorsToTrackTime = sec.Key("DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME").MustBool(true)
	Service.NoReplyAddress = sec.Key("NO_REPLY_ADDRESS").MustString("noreply." + Domain)
	Service.EnableUserHeatmap = sec.Key("ENABLE_USER_HEATMAP").MustBool(true)
	Service.AutoWatchNewRepos = sec.Key("AUTO_WATCH_NEW_REPOS").MustBool(true)
	Service.AutoWatchOnChanges = sec.Key("AUTO_WATCH_ON_CHANGES").MustBool(false)
	Service.DefaultOrgVisibility = sec.Key("DEFAULT_ORG_VISIBILITY").In("public", structs.ExtractKeysFromMapString(structs.VisibilityModes))
	Service.DefaultOrgVisibilityMode = structs.VisibilityModes[Service.DefaultOrgVisibility]
	Service.DefaultOrgMemberVisible = sec.Key("DEFAULT_ORG_MEMBER_VISIBLE").MustBool()
	Service.UserDeleteWithCommentsMaxTime = sec.Key("USER_DELETE_WITH_COMMENTS_MAX_TIME").MustDuration(0)

	if err := Cfg.Section("service.explore").MapTo(&Service.Explore); err != nil {
		log.Fatal("Failed to map service.explore settings: %v", err)
	}

	sec = Cfg.Section("openid")
	Service.EnableOpenIDSignIn = sec.Key("ENABLE_OPENID_SIGNIN").MustBool(!InstallLock)
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

	sec = Cfg.Section("oauth2_client")
	OAuth2Client.RegisterEmailConfirm = sec.Key("REGISTER_EMAIL_CONFIRM").MustBool(Service.RegisterEmailConfirm)
	OAuth2Client.OpenIDConnectScopes = parseScopes(sec, "OPENID_CONNECT_SCOPES")
	OAuth2Client.GoogleScopes = parseScopes(sec, "GOOGLE_SCOPES")
	OAuth2Client.EnableAutoRegistration = sec.Key("ENABLE_AUTO_REGISTRATION").MustBool()
	OAuth2Client.Username = sec.Key("USERNAME").MustString("userid")
	if !isValidUsername(OAuth2Client.Username) {
		log.Warn("Username setting is not valid: '%s', will fallback to 'userid'", OAuth2Client.Username)
		OAuth2Client.Username = "userid"
	}
	OAuth2Client.UpdateAvatar = sec.Key("UPDATE_AVATAR").MustBool()
	OAuth2Client.AccountLinking = sec.Key("ACCOUNT_LINKING").MustString("ACCOUNT_LINKING")
	if !isValidAccountLinking(OAuth2Client.AccountLinking) {
		log.Warn("Account linking setting is not valid: '%s', will fallback to 'disabled'")
		OAuth2Client.AccountLinking = "disabled"
	}
}

func isValidUsername(username string) bool {
	switch username {
	case "userid":
	case "nickname":
	case "email":
		return true
	}
	return false
}

func isValidAccountLinking(accountLinking string) bool {
	switch accountLinking {
	case "disabled":
	case "login":
	case "auto":
		return true
	}
	return false
}

func parseScopes(sec *ini.Section, name string) []string {
	parts := sec.Key(name).Strings(" ")
	scopes := make([]string, 0, len(parts))
	for _, scope :=	 range parts {
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}
