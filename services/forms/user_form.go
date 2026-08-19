// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"mime/multipart"
	"strings"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"
	"gitea.dev/modules/web/middleware"
)

// InstallForm form for installation page
type InstallForm struct {
	middleware.FormDefaultValidator
	DbType   string `binding:"TrimSpace;Required"`
	DbHost   string `binding:"TrimSpace"`
	DbUser   string `binding:"TrimSpace"`
	DbPasswd string
	DbName   string `binding:"TrimSpace"`
	SSLMode  string `binding:"TrimSpace"`
	DbPath   string `binding:"TrimSpace"`
	DbSchema string `binding:"TrimSpace"`

	AppName      string `binding:"TrimSpace;Required" locale:"install.app_name"`
	RepoRootPath string `binding:"TrimSpace;Required"`
	LFSRootPath  string `binding:"TrimSpace"`
	RunUser      string `binding:"TrimSpace;Required"`
	Domain       string `binding:"TrimSpace;Required"`
	SSHPort      int
	HTTPPort     string `binding:"TrimSpace;Required"`
	AppURL       string `binding:"TrimSpace;Required"`
	LogRootPath  string `binding:"TrimSpace;Required"`

	SMTPAddr        string `binding:"TrimSpace"`
	SMTPPort        string `binding:"TrimSpace"`
	SMTPFrom        string `binding:"TrimSpace"`
	SMTPUser        string `binding:"TrimSpace;OmitEmpty;MaxSize(254)" locale:"install.mailer_user"`
	SMTPPasswd      string
	RegisterConfirm bool
	MailNotify      bool

	EnableOpenIDSignIn             bool
	EnableOpenIDSignUp             bool
	DisableRegistration            bool
	AllowOnlyExternalRegistration  bool
	EnableCaptcha                  bool
	RequireSignInView              bool
	DefaultKeepEmailPrivate        bool
	DefaultAllowCreateOrganization bool
	DefaultEnableTimetracking      bool
	EnableUpdateChecker            bool
	NoReplyAddress                 string `binding:"TrimSpace"`

	PasswordAlgorithm string `binding:"TrimSpace"`

	AdminName          string `binding:"TrimSpace;OmitEmpty;Username;MaxSize(30)" locale:"install.admin_name"`
	AdminPasswd        string `binding:"OmitEmpty;MaxSize(255)" locale:"install.admin_password"`
	AdminConfirmPasswd string
	AdminEmail         string `binding:"TrimSpace;OmitEmpty;MinSize(3);MaxSize(254);Include(@)" locale:"install.admin_email"`

	// ReinstallConfirmFirst we can not use 1/2/3 or A/B/C here, there is a framework bug, can not parse "reinstall_confirm_1" or "reinstall_confirm_a"
	ReinstallConfirmFirst  bool
	ReinstallConfirmSecond bool
	ReinstallConfirmThird  bool
}

//    _____   ____ _________________ ___
//   /  _  \ |    |   \__    ___/   |   \
//  /  /_\  \|    |   / |    | /    ~    \
// /    |    \    |  /  |    | \    Y    /
// \____|__  /______/   |____|  \___|_  /
//         \/                         \/

// RegisterForm form for registering
type RegisterForm struct {
	middleware.FormDefaultValidator
	UserName string `binding:"Required;Username;MaxSize(40)"`
	Email    string `binding:"Required;MaxSize(254)"`
	Password string `binding:"MaxSize(255)"`
	Retype   string
}

// IsEmailDomainAllowed validates that the email address
// provided by the user matches what has been configured .
// The email is marked as allowed if it matches any of the
// domains in the whitelist or if it doesn't match any of
// domains in the blocklist, if any such list is not empty.
func (f *RegisterForm) IsEmailDomainAllowed() bool {
	return user_model.IsEmailDomainAllowed(f.Email)
}

// MustChangePasswordForm form for updating your password after account creation
// by an admin
type MustChangePasswordForm struct {
	middleware.FormDefaultValidator
	Password string `binding:"Required;MaxSize(255)"`
	Retype   string
}

// SignInForm form for signing in with user/password
type SignInForm struct {
	middleware.FormDefaultValidator
	UserName string `binding:"Required;MaxSize(254)"`
	// TODO remove required from password for SecondFactorAuthentication
	Password string `binding:"Required;MaxSize(255)"`
	Remember bool
}

// AuthorizationForm form for authorizing oauth2 clients
type AuthorizationForm struct {
	middleware.FormDefaultValidator
	ResponseType string
	ClientID     string
	RedirectURI  string
	State        string
	Scope        string
	Nonce        string

	// PKCE support
	CodeChallengeMethod string // S256, plain
	CodeChallenge       string
}

// GrantApplicationForm form for authorizing oauth2 clients
type GrantApplicationForm struct {
	middleware.FormDefaultValidator
	ClientID    string `binding:"Required"`
	Granted     bool
	RedirectURI string
	State       string
	Scope       string
	Nonce       string
}

// AccessTokenForm for issuing access tokens from authorization codes or refresh tokens
type AccessTokenForm struct {
	middleware.FormDefaultValidator
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	Code         string `json:"code"`
	RefreshToken string `json:"refresh_token"`

	// PKCE support
	CodeVerifier string `json:"code_verifier"`
}

// IntrospectTokenForm for introspecting tokens
type IntrospectTokenForm struct {
	middleware.FormDefaultValidator
	Token string `json:"token"`
}

//   __________________________________________.___ _______    ________  _________
//  /   _____/\_   _____/\__    ___/\__    ___/|   |\      \  /  _____/ /   _____/
//  \_____  \  |    __)_   |    |     |    |   |   |/   |   \/   \  ___ \_____  \
//  /        \ |        \  |    |     |    |   |   /    |    \    \_\  \/        \
// /_______  //_______  /  |____|     |____|   |___\____|__  /\______  /_______  /
//         \/         \/                                   \/        \/        \/

// UpdateProfileForm form for updating profile
type UpdateProfileForm struct {
	middleware.FormDefaultValidator
	Name                string `binding:"Username;MaxSize(40)"`
	FullName            string `binding:"MaxSize(100)"`
	KeepEmailPrivate    bool
	Website             string `binding:"ValidSiteUrl;MaxSize(255)"`
	Location            string `binding:"MaxSize(50)"`
	Description         string `binding:"MaxSize(255)"`
	Visibility          structs.VisibleType
	KeepActivityPrivate bool
}

// UpdateLanguageForm form for updating profile
type UpdateLanguageForm struct {
	middleware.FormDefaultValidator
	Language string
}

const AvatarLocal = "local" // the AvatarForm.Source value that selects an uploaded avatar

// AvatarForm form for changing avatar
type AvatarForm struct {
	middleware.FormDefaultValidator
	Source   string
	Avatar   *multipart.FileHeader
	Gravatar string `binding:"OmitEmpty;Email;MaxSize(254)"`
}

// AddEmailForm form for adding new email
type AddEmailForm struct {
	middleware.FormDefaultValidator
	Email string `binding:"Required;Email;MaxSize(254)"`
}

// UpdateThemeForm form for updating a users' theme
type UpdateThemeForm struct {
	middleware.FormDefaultValidator
	Theme string `binding:"Required;MaxSize(255)"`
}

// ChangePasswordForm form for changing password
type ChangePasswordForm struct {
	middleware.FormDefaultValidator
	OldPassword string `form:"old_password" binding:"MaxSize(255)"`
	Password    string `form:"password" binding:"Required;MaxSize(255)"`
	Retype      string `form:"retype"`
}

// AddOpenIDForm is for changing openid uri
type AddOpenIDForm struct {
	middleware.FormDefaultValidator
	Openid string `binding:"Required;MaxSize(256)"`
}

// AddKeyForm form for adding SSH/GPG key
type AddKeyForm struct {
	middleware.FormDefaultValidator
	Type        string `binding:"OmitEmpty"`
	Title       string `binding:"Required;MaxSize(50)"`
	Content     string `binding:"Required"`
	Signature   string `binding:"OmitEmpty"`
	KeyID       string `binding:"OmitEmpty"`
	Fingerprint string `binding:"OmitEmpty"`
	IsWritable  bool
}

// AddSecretForm for adding secrets
type AddSecretForm struct {
	middleware.FormDefaultValidator
	Name        string `binding:"Required;MaxSize(255)"`
	Data        string `binding:"Required;MaxSize(65535)"`
	Description string `binding:"MaxSize(65535)"`
}

type EditVariableForm struct {
	middleware.FormDefaultValidator
	Name        string `binding:"Required;MaxSize(255)"`
	Data        string `binding:"Required;MaxSize(65535)"`
	Description string `binding:"MaxSize(65535)"`
}

// NewAccessTokenForm form for creating access token
type NewAccessTokenForm struct {
	middleware.FormDefaultValidator
	Name string `binding:"Required;MaxSize(255)" locale:"settings.token_name"`
}

// EditOAuth2ApplicationForm form for editing oauth2 applications
type EditOAuth2ApplicationForm struct {
	Name                       string `binding:"Required;MaxSize(255)" form:"application_name"`
	RedirectURIs               string `binding:"Required" form:"redirect_uris"`
	ConfidentialClient         bool   `form:"confidential_client"`
	SkipSecondaryAuthorization bool   `form:"skip_secondary_authorization"`
}

func DetectInvalidOAuth2ApplicationRedirectURI(uris []string) (invalidURL string) {
	for _, u := range uris {
		scheme, _, ok := strings.Cut(u, ":")
		valid := ok && (validation.IsValidURL(u) || util.SliceContainsString(setting.OAuth2.CustomSchemes, scheme))
		if !valid {
			return u
		}
	}
	return ""
}

func (f *EditOAuth2ApplicationForm) Validate(ctx *middleware.ValidateContext, errs validation.BindingErrors) validation.BindingErrors {
	invalidURI := DetectInvalidOAuth2ApplicationRedirectURI(util.SplitTrimSpace(f.RedirectURIs, "\n"))
	if invalidURI != "" {
		errs = middleware.AddValidationError(errs, "RedirectURIs", "RedirectURIs: "+ctx.Locale.TrString("form.url_error", `"`+invalidURI+`"`))
	}
	return errs
}

// TwoFactorAuthForm for logging in with 2FA token.
type TwoFactorAuthForm struct {
	middleware.FormDefaultValidator
	Passcode string `binding:"Required"`
}

// TwoFactorScratchAuthForm for logging in with 2FA scratch token.
type TwoFactorScratchAuthForm struct {
	middleware.FormDefaultValidator
	Token string `binding:"Required"`
}

// WebauthnRegistrationForm for reserving an WebAuthn name
type WebauthnRegistrationForm struct {
	middleware.FormDefaultValidator
	Name string `binding:"Required"`
}

// PackageSettingForm form for package settings
type PackageSettingForm struct {
	middleware.FormDefaultValidator
	Action   string
	RepoName string `form:"repo_name"`
}

type BlockUserForm struct {
	middleware.FormDefaultValidator
	Action  string `binding:"Required;In(block,unblock,note)"`
	Blockee string `binding:"Required"`
	Note    string
}
