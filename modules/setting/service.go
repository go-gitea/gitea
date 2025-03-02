// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"

	"github.com/gobwas/glob"
)

// enumerates all the types of captchas
const (
	ImageCaptcha = "image"
	ReCaptcha    = "recaptcha"
	HCaptcha     = "hcaptcha"
	MCaptcha     = "mcaptcha"
	CfTurnstile  = "cfturnstile"
)

// Service settings
var Service = struct {
	DefaultUserVisibility                   string
	DefaultUserVisibilityMode               structs.VisibleType
	AllowedUserVisibilityModes              []string
	AllowedUserVisibilityModesSlice         AllowedVisibility `ini:"-"`
	DefaultOrgVisibility                    string
	DefaultOrgVisibilityMode                structs.VisibleType
	ActiveCodeLives                         int
	ResetPwdCodeLives                       int
	RegisterEmailConfirm                    bool
	RegisterManualConfirm                   bool
	EmailDomainAllowList                    []glob.Glob
	EmailDomainBlockList                    []glob.Glob
	DisableRegistration                     bool
	AllowOnlyInternalRegistration           bool
	AllowOnlyExternalRegistration           bool
	ShowRegistrationButton                  bool
	EnablePasswordSignInForm                bool
	ShowMilestonesDashboardPage             bool
	RequireSignInView                       bool
	EnableNotifyMail                        bool
	EnableBasicAuth                         bool
	EnablePasskeyAuth                       bool
	EnableReverseProxyAuth                  bool
	EnableReverseProxyAuthAPI               bool
	EnableReverseProxyAutoRegister          bool
	EnableReverseProxyEmail                 bool
	EnableReverseProxyFullName              bool
	EnableCaptcha                           bool
	RequireCaptchaForLogin                  bool
	RequireExternalRegistrationCaptcha      bool
	RequireExternalRegistrationPassword     bool
	CaptchaType                             string
	RecaptchaSecret                         string
	RecaptchaSitekey                        string
	RecaptchaURL                            string
	CfTurnstileSecret                       string
	CfTurnstileSitekey                      string
	HcaptchaSecret                          string
	HcaptchaSitekey                         string
	McaptchaSecret                          string
	McaptchaSitekey                         string
	McaptchaURL                             string
	DefaultKeepEmailPrivate                 bool
	DefaultAllowCreateOrganization          bool
	DefaultUserIsRestricted                 bool
	EnableTimetracking                      bool
	DefaultEnableTimetracking               bool
	DefaultEnableDependencies               bool
	AllowCrossRepositoryDependencies        bool
	DefaultAllowOnlyContributorsToTrackTime bool
	NoReplyAddress                          string
	UserLocationMapURL                      string
	EnableUserHeatmap                       bool
	AutoWatchNewRepos                       bool
	AutoWatchOnChanges                      bool
	DefaultOrgMemberVisible                 bool
	UserDeleteWithCommentsMaxTime           time.Duration
	ValidSiteURLSchemes                     []string

	// OpenID settings
	EnableOpenIDSignIn bool
	EnableOpenIDSignUp bool
	OpenIDWhitelist    []*regexp.Regexp
	OpenIDBlacklist    []*regexp.Regexp

	// Explore page settings
	Explore struct {
		RequireSigninView        bool `ini:"REQUIRE_SIGNIN_VIEW"`
		DisableUsersPage         bool `ini:"DISABLE_USERS_PAGE"`
		DisableOrganizationsPage bool `ini:"DISABLE_ORGANIZATIONS_PAGE"`
		DisableCodePage          bool `ini:"DISABLE_CODE_PAGE"`
	} `ini:"service.explore"`
}{
	AllowedUserVisibilityModesSlice: []bool{true, true, true},
}

// AllowedVisibility store in a 3 item bool array what is allowed
type AllowedVisibility []bool

// IsAllowedVisibility check if a AllowedVisibility allow a specific VisibleType
func (a AllowedVisibility) IsAllowedVisibility(t structs.VisibleType) bool {
	if int(t) >= len(a) {
		return false
	}
	return a[t]
}

// ToVisibleTypeSlice convert a AllowedVisibility into a VisibleType slice
func (a AllowedVisibility) ToVisibleTypeSlice() (result []structs.VisibleType) {
	for i, v := range a {
		if v {
			result = append(result, structs.VisibleType(i))
		}
	}
	return result
}

func CompileEmailGlobList(sec ConfigSection, keys ...string) (globs []glob.Glob) {
	for _, key := range keys {
		list := sec.Key(key).Strings(",")
		for _, s := range list {
			if g, err := glob.Compile(s); err == nil {
				globs = append(globs, g)
			} else {
				log.Error("Skip invalid email allow/block list expression %q: %v", s, err)
			}
		}
	}
	return globs
}

func loadServiceFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("service")
	Service.ActiveCodeLives = sec.Key("ACTIVE_CODE_LIVE_MINUTES").MustInt(180)
	Service.ResetPwdCodeLives = sec.Key("RESET_PASSWD_CODE_LIVE_MINUTES").MustInt(180)
	Service.DisableRegistration = sec.Key("DISABLE_REGISTRATION").MustBool()
	Service.AllowOnlyInternalRegistration = sec.Key("ALLOW_ONLY_INTERNAL_REGISTRATION").MustBool()
	Service.AllowOnlyExternalRegistration = sec.Key("ALLOW_ONLY_EXTERNAL_REGISTRATION").MustBool()
	if Service.AllowOnlyExternalRegistration && Service.AllowOnlyInternalRegistration {
		log.Warn("ALLOW_ONLY_INTERNAL_REGISTRATION and ALLOW_ONLY_EXTERNAL_REGISTRATION are true - disabling registration")
		Service.DisableRegistration = true
	}
	if !sec.Key("REGISTER_EMAIL_CONFIRM").MustBool() {
		Service.RegisterManualConfirm = sec.Key("REGISTER_MANUAL_CONFIRM").MustBool(false)
	} else {
		Service.RegisterManualConfirm = false
	}
	if sec.HasKey("EMAIL_DOMAIN_WHITELIST") {
		deprecatedSetting(rootCfg, "service", "EMAIL_DOMAIN_WHITELIST", "service", "EMAIL_DOMAIN_ALLOWLIST", "1.21")
	}
	Service.EmailDomainAllowList = CompileEmailGlobList(sec, "EMAIL_DOMAIN_WHITELIST", "EMAIL_DOMAIN_ALLOWLIST")
	Service.EmailDomainBlockList = CompileEmailGlobList(sec, "EMAIL_DOMAIN_BLOCKLIST")
	Service.ShowRegistrationButton = sec.Key("SHOW_REGISTRATION_BUTTON").MustBool(!(Service.DisableRegistration || Service.AllowOnlyExternalRegistration))
	Service.ShowMilestonesDashboardPage = sec.Key("SHOW_MILESTONES_DASHBOARD_PAGE").MustBool(true)
	Service.RequireSignInView = sec.Key("REQUIRE_SIGNIN_VIEW").MustBool()
	Service.EnableBasicAuth = sec.Key("ENABLE_BASIC_AUTHENTICATION").MustBool(true)
	Service.EnablePasswordSignInForm = sec.Key("ENABLE_PASSWORD_SIGNIN_FORM").MustBool(true)
	Service.EnablePasskeyAuth = sec.Key("ENABLE_PASSKEY_AUTHENTICATION").MustBool(true)
	Service.EnableReverseProxyAuth = sec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	Service.EnableReverseProxyAuthAPI = sec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION_API").MustBool()
	Service.EnableReverseProxyAutoRegister = sec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	Service.EnableReverseProxyEmail = sec.Key("ENABLE_REVERSE_PROXY_EMAIL").MustBool()
	Service.EnableReverseProxyFullName = sec.Key("ENABLE_REVERSE_PROXY_FULL_NAME").MustBool()
	Service.EnableCaptcha = sec.Key("ENABLE_CAPTCHA").MustBool(false)
	Service.RequireCaptchaForLogin = sec.Key("REQUIRE_CAPTCHA_FOR_LOGIN").MustBool(false)
	Service.RequireExternalRegistrationCaptcha = sec.Key("REQUIRE_EXTERNAL_REGISTRATION_CAPTCHA").MustBool(Service.EnableCaptcha)
	Service.RequireExternalRegistrationPassword = sec.Key("REQUIRE_EXTERNAL_REGISTRATION_PASSWORD").MustBool()
	Service.CaptchaType = sec.Key("CAPTCHA_TYPE").MustString(ImageCaptcha)
	Service.RecaptchaSecret = sec.Key("RECAPTCHA_SECRET").MustString("")
	Service.RecaptchaSitekey = sec.Key("RECAPTCHA_SITEKEY").MustString("")
	Service.RecaptchaURL = sec.Key("RECAPTCHA_URL").MustString("https://www.google.com/recaptcha/")
	Service.CfTurnstileSecret = sec.Key("CF_TURNSTILE_SECRET").MustString("")
	Service.CfTurnstileSitekey = sec.Key("CF_TURNSTILE_SITEKEY").MustString("")
	Service.HcaptchaSecret = sec.Key("HCAPTCHA_SECRET").MustString("")
	Service.HcaptchaSitekey = sec.Key("HCAPTCHA_SITEKEY").MustString("")
	Service.McaptchaURL = sec.Key("MCAPTCHA_URL").MustString("https://demo.mcaptcha.org/")
	Service.McaptchaSecret = sec.Key("MCAPTCHA_SECRET").MustString("")
	Service.McaptchaSitekey = sec.Key("MCAPTCHA_SITEKEY").MustString("")
	Service.DefaultKeepEmailPrivate = sec.Key("DEFAULT_KEEP_EMAIL_PRIVATE").MustBool()
	Service.DefaultAllowCreateOrganization = sec.Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").MustBool(true)
	Service.DefaultUserIsRestricted = sec.Key("DEFAULT_USER_IS_RESTRICTED").MustBool(false)
	Service.EnableTimetracking = sec.Key("ENABLE_TIMETRACKING").MustBool(true)
	if Service.EnableTimetracking {
		Service.DefaultEnableTimetracking = sec.Key("DEFAULT_ENABLE_TIMETRACKING").MustBool(true)
	}
	Service.DefaultEnableDependencies = sec.Key("DEFAULT_ENABLE_DEPENDENCIES").MustBool(true)
	Service.AllowCrossRepositoryDependencies = sec.Key("ALLOW_CROSS_REPOSITORY_DEPENDENCIES").MustBool(true)
	Service.DefaultAllowOnlyContributorsToTrackTime = sec.Key("DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME").MustBool(true)
	Service.NoReplyAddress = sec.Key("NO_REPLY_ADDRESS").MustString("noreply." + Domain)
	Service.UserLocationMapURL = sec.Key("USER_LOCATION_MAP_URL").String()
	Service.EnableUserHeatmap = sec.Key("ENABLE_USER_HEATMAP").MustBool(true)
	Service.AutoWatchNewRepos = sec.Key("AUTO_WATCH_NEW_REPOS").MustBool(true)
	Service.AutoWatchOnChanges = sec.Key("AUTO_WATCH_ON_CHANGES").MustBool(false)
	modes := sec.Key("ALLOWED_USER_VISIBILITY_MODES").Strings(",")
	if len(modes) != 0 {
		Service.AllowedUserVisibilityModes = []string{}
		Service.AllowedUserVisibilityModesSlice = []bool{false, false, false}
		for _, sMode := range modes {
			if tp, ok := structs.VisibilityModes[sMode]; ok { // remove unsupported modes
				Service.AllowedUserVisibilityModes = append(Service.AllowedUserVisibilityModes, sMode)
				Service.AllowedUserVisibilityModesSlice[tp] = true
			} else {
				log.Warn("ALLOWED_USER_VISIBILITY_MODES %s is unsupported", sMode)
			}
		}
	}

	if len(Service.AllowedUserVisibilityModes) == 0 {
		Service.AllowedUserVisibilityModes = []string{"public", "limited", "private"}
		Service.AllowedUserVisibilityModesSlice = []bool{true, true, true}
	}

	Service.DefaultUserVisibility = sec.Key("DEFAULT_USER_VISIBILITY").String()
	if Service.DefaultUserVisibility == "" {
		Service.DefaultUserVisibility = Service.AllowedUserVisibilityModes[0]
	} else if !Service.AllowedUserVisibilityModesSlice[structs.VisibilityModes[Service.DefaultUserVisibility]] {
		log.Warn("DEFAULT_USER_VISIBILITY %s is wrong or not in ALLOWED_USER_VISIBILITY_MODES, using first allowed", Service.DefaultUserVisibility)
		Service.DefaultUserVisibility = Service.AllowedUserVisibilityModes[0]
	}
	Service.DefaultUserVisibilityMode = structs.VisibilityModes[Service.DefaultUserVisibility]
	Service.DefaultOrgVisibility = sec.Key("DEFAULT_ORG_VISIBILITY").In("public", structs.ExtractKeysFromMapString(structs.VisibilityModes))
	Service.DefaultOrgVisibilityMode = structs.VisibilityModes[Service.DefaultOrgVisibility]
	Service.DefaultOrgMemberVisible = sec.Key("DEFAULT_ORG_MEMBER_VISIBLE").MustBool()
	Service.UserDeleteWithCommentsMaxTime = sec.Key("USER_DELETE_WITH_COMMENTS_MAX_TIME").MustDuration(0)
	sec.Key("VALID_SITE_URL_SCHEMES").MustString("http,https")
	Service.ValidSiteURLSchemes = sec.Key("VALID_SITE_URL_SCHEMES").Strings(",")
	schemes := make([]string, 0, len(Service.ValidSiteURLSchemes))
	for _, scheme := range Service.ValidSiteURLSchemes {
		scheme = strings.ToLower(strings.TrimSpace(scheme))
		if scheme != "" {
			schemes = append(schemes, scheme)
		}
	}
	Service.ValidSiteURLSchemes = schemes

	mustMapSetting(rootCfg, "service.explore", &Service.Explore)

	loadOpenIDSetting(rootCfg)
}

func loadOpenIDSetting(rootCfg ConfigProvider) {
	sec := rootCfg.Section("openid")
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
