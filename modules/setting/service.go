// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"regexp"

	"code.gitea.io/gitea/modules/structs"
)

// Service settings
var Service struct {
	DefaultOrgVisibility                    string
	DefaultOrgVisibilityMode                structs.VisibleType
	ActiveCodeLives                         int
	ResetPwdCodeLives                       int
	RegisterEmailConfirm                    bool
	EmailDomainWhitelist                    []string
	DisableRegistration                     bool
	AllowOnlyExternalRegistration           bool
	ShowRegistrationButton                  bool
	RequireSignInView                       bool
	EnableNotifyMail                        bool
	EnableReverseProxyAuth                  bool
	EnableReverseProxyAutoRegister          bool
	EnableReverseProxyEmail                 bool
	EnableCaptcha                           bool
	CaptchaType                             string
	RecaptchaSecret                         string
	RecaptchaSitekey                        string
	DefaultKeepEmailPrivate                 bool
	DefaultAllowCreateOrganization          bool
	EnableTimetracking                      bool
	DefaultEnableTimetracking               bool
	DefaultEnableDependencies               bool
	DefaultAllowOnlyContributorsToTrackTime bool
	NoReplyAddress                          string
	EnableUserHeatmap                       bool
	AutoWatchNewRepos                       bool

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
	Service.AllowOnlyExternalRegistration = sec.Key("ALLOW_ONLY_EXTERNAL_REGISTRATION").MustBool()
	Service.EmailDomainWhitelist = sec.Key("EMAIL_DOMAIN_WHITELIST").Strings(",")
	Service.ShowRegistrationButton = sec.Key("SHOW_REGISTRATION_BUTTON").MustBool(!(Service.DisableRegistration || Service.AllowOnlyExternalRegistration))
	Service.RequireSignInView = sec.Key("REQUIRE_SIGNIN_VIEW").MustBool()
	Service.EnableReverseProxyAuth = sec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	Service.EnableReverseProxyAutoRegister = sec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	Service.EnableReverseProxyEmail = sec.Key("ENABLE_REVERSE_PROXY_EMAIL").MustBool()
	Service.EnableCaptcha = sec.Key("ENABLE_CAPTCHA").MustBool(false)
	Service.CaptchaType = sec.Key("CAPTCHA_TYPE").MustString(ImageCaptcha)
	Service.RecaptchaSecret = sec.Key("RECAPTCHA_SECRET").MustString("")
	Service.RecaptchaSitekey = sec.Key("RECAPTCHA_SITEKEY").MustString("")
	Service.DefaultKeepEmailPrivate = sec.Key("DEFAULT_KEEP_EMAIL_PRIVATE").MustBool()
	Service.DefaultAllowCreateOrganization = sec.Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").MustBool(true)
	Service.EnableTimetracking = sec.Key("ENABLE_TIMETRACKING").MustBool(true)
	if Service.EnableTimetracking {
		Service.DefaultEnableTimetracking = sec.Key("DEFAULT_ENABLE_TIMETRACKING").MustBool(true)
	}
	Service.DefaultEnableDependencies = sec.Key("DEFAULT_ENABLE_DEPENDENCIES").MustBool(true)
	Service.DefaultAllowOnlyContributorsToTrackTime = sec.Key("DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME").MustBool(true)
	Service.NoReplyAddress = sec.Key("NO_REPLY_ADDRESS").MustString("noreply.example.org")
	Service.EnableUserHeatmap = sec.Key("ENABLE_USER_HEATMAP").MustBool(true)
	Service.AutoWatchNewRepos = sec.Key("AUTO_WATCH_NEW_REPOS").MustBool(true)
	Service.DefaultOrgVisibility = sec.Key("DEFAULT_ORG_VISIBILITY").In("public", structs.ExtractKeysFromMapString(structs.VisibilityModes))
	Service.DefaultOrgVisibilityMode = structs.VisibilityModes[Service.DefaultOrgVisibility]

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
}
