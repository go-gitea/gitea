// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"mime/multipart"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/binding"
)

// InstallForm form for installation page
type InstallForm struct {
	DbType   string `binding:"Required"`
	DbHost   string
	DbUser   string
	DbPasswd string
	DbName   string
	SSLMode  string
	DbPath   string
	DbSchema string

	AppName      string `binding:"Required" locale:"install.app_name"`
	RepoRootPath string `binding:"Required"`
	LFSRootPath  string
	RunUser      string `binding:"Required"`
	Domain       string `binding:"Required"`
	SSHPort      int
	HTTPPort     string `binding:"Required"`
	AppURL       string `binding:"Required"`
	LogRootPath  string `binding:"Required"`

	SMTPAddr        string
	SMTPPort        string
	SMTPFrom        string
	SMTPUser        string `binding:"OmitEmpty;MaxSize(254)" locale:"install.mailer_user"`
	SMTPPasswd      string
	RegisterConfirm bool
	MailNotify      bool

	OfflineMode                    bool
	DisableGravatar                bool
	EnableFederatedAvatar          bool
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
	NoReplyAddress                 string

	PasswordAlgorithm string

	AdminName          string `binding:"OmitEmpty;Username;MaxSize(30)" locale:"install.admin_name"`
	AdminPasswd        string `binding:"OmitEmpty;MaxSize(255)" locale:"install.admin_password"`
	AdminConfirmPasswd string
	AdminEmail         string `binding:"OmitEmpty;MinSize(3);MaxSize(254);Include(@)" locale:"install.admin_email"`

	// ReinstallConfirmFirst we can not use 1/2/3 or A/B/C here, there is a framework bug, can not parse "reinstall_confirm_1" or "reinstall_confirm_a"
	ReinstallConfirmFirst  bool
	ReinstallConfirmSecond bool
	ReinstallConfirmThird  bool
}

// Validate validates the fields
func (f *InstallForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

//    _____   ____ _________________ ___
//   /  _  \ |    |   \__    ___/   |   \
//  /  /_\  \|    |   / |    | /    ~    \
// /    |    \    |  /  |    | \    Y    /
// \____|__  /______/   |____|  \___|_  /
//         \/                         \/

// RegisterForm form for registering
type RegisterForm struct {
	UserName string `binding:"Required;Username;MaxSize(40)"`
	Email    string `binding:"Required;MaxSize(254)"`
	Password string `binding:"MaxSize(255)"`
	Retype   string
}

// Validate validates the fields
func (f *RegisterForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
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
	Password string `binding:"Required;MaxSize(255)"`
	Retype   string
}

// Validate validates the fields
func (f *MustChangePasswordForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// SignInForm form for signing in with user/password
type SignInForm struct {
	UserName string `binding:"Required;MaxSize(254)"`
	// TODO remove required from password for SecondFactorAuthentication
	Password string `binding:"Required;MaxSize(255)"`
	Remember bool
}

// Validate validates the fields
func (f *SignInForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AuthorizationForm form for authorizing oauth2 clients
type AuthorizationForm struct {
	ResponseType string `binding:"Required;In(code)"`
	ClientID     string `binding:"Required"`
	RedirectURI  string
	State        string
	Scope        string
	Nonce        string

	// PKCE support
	CodeChallengeMethod string // S256, plain
	CodeChallenge       string
}

// Validate validates the fields
func (f *AuthorizationForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// GrantApplicationForm form for authorizing oauth2 clients
type GrantApplicationForm struct {
	ClientID    string `binding:"Required"`
	Granted     bool
	RedirectURI string
	State       string
	Scope       string
	Nonce       string
}

// Validate validates the fields
func (f *GrantApplicationForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AccessTokenForm for issuing access tokens from authorization codes or refresh tokens
type AccessTokenForm struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	Code         string `json:"code"`
	RefreshToken string `json:"refresh_token"`

	// PKCE support
	CodeVerifier string `json:"code_verifier"`
}

// Validate validates the fields
func (f *AccessTokenForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// IntrospectTokenForm for introspecting tokens
type IntrospectTokenForm struct {
	Token string `json:"token"`
}

// Validate validates the fields
func (f *IntrospectTokenForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

//   __________________________________________.___ _______    ________  _________
//  /   _____/\_   _____/\__    ___/\__    ___/|   |\      \  /  _____/ /   _____/
//  \_____  \  |    __)_   |    |     |    |   |   |/   |   \/   \  ___ \_____  \
//  /        \ |        \  |    |     |    |   |   /    |    \    \_\  \/        \
// /_______  //_______  /  |____|     |____|   |___\____|__  /\______  /_______  /
//         \/         \/                                   \/        \/        \/

// UpdateProfileForm form for updating profile
type UpdateProfileForm struct {
	Name                string `binding:"Username;MaxSize(40)"`
	FullName            string `binding:"MaxSize(100)"`
	KeepEmailPrivate    bool
	Website             string `binding:"ValidSiteUrl;MaxSize(255)"`
	Location            string `binding:"MaxSize(50)"`
	Description         string `binding:"MaxSize(255)"`
	Visibility          structs.VisibleType
	KeepActivityPrivate bool
}

// Validate validates the fields
func (f *UpdateProfileForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// UpdateLanguageForm form for updating profile
type UpdateLanguageForm struct {
	Language string
}

// Validate validates the fields
func (f *UpdateLanguageForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// Avatar types
const (
	AvatarLocal  string = "local"
	AvatarByMail string = "bymail"
)

// AvatarForm form for changing avatar
type AvatarForm struct {
	Source      string
	Avatar      *multipart.FileHeader
	Gravatar    string `binding:"OmitEmpty;Email;MaxSize(254)"`
	Federavatar bool
}

// Validate validates the fields
func (f *AvatarForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AddEmailForm form for adding new email
type AddEmailForm struct {
	Email string `binding:"Required;Email;MaxSize(254)"`
}

// Validate validates the fields
func (f *AddEmailForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// UpdateThemeForm form for updating a users' theme
type UpdateThemeForm struct {
	Theme string `binding:"Required;MaxSize(255)"`
}

// Validate validates the field
func (f *UpdateThemeForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// ChangePasswordForm form for changing password
type ChangePasswordForm struct {
	OldPassword string `form:"old_password" binding:"MaxSize(255)"`
	Password    string `form:"password" binding:"Required;MaxSize(255)"`
	Retype      string `form:"retype"`
}

// Validate validates the fields
func (f *ChangePasswordForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AddOpenIDForm is for changing openid uri
type AddOpenIDForm struct {
	Openid string `binding:"Required;MaxSize(256)"`
}

// Validate validates the fields
func (f *AddOpenIDForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AddKeyForm form for adding SSH/GPG key
type AddKeyForm struct {
	Type        string `binding:"OmitEmpty"`
	Title       string `binding:"Required;MaxSize(50)"`
	Content     string `binding:"Required"`
	Signature   string `binding:"OmitEmpty"`
	KeyID       string `binding:"OmitEmpty"`
	Fingerprint string `binding:"OmitEmpty"`
	IsWritable  bool
}

// Validate validates the fields
func (f *AddKeyForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AddSecretForm for adding secrets
type AddSecretForm struct {
	Name        string `binding:"Required;MaxSize(255)"`
	Data        string `binding:"Required;MaxSize(65535)"`
	Description string `binding:"MaxSize(65535)"`
}

// Validate validates the fields
func (f *AddSecretForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

type EditVariableForm struct {
	Name        string `binding:"Required;MaxSize(255)"`
	Data        string `binding:"Required;MaxSize(65535)"`
	Description string `binding:"MaxSize(65535)"`
}

func (f *EditVariableForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// NewAccessTokenForm form for creating access token
type NewAccessTokenForm struct {
	Name string `binding:"Required;MaxSize(255)" locale:"settings.token_name"`
}

// Validate validates the fields
func (f *NewAccessTokenForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// EditOAuth2ApplicationForm form for editing oauth2 applications
type EditOAuth2ApplicationForm struct {
	Name                       string `binding:"Required;MaxSize(255)" form:"application_name"`
	RedirectURIs               string `binding:"Required;ValidUrlList" form:"redirect_uris"`
	ConfidentialClient         bool   `form:"confidential_client"`
	SkipSecondaryAuthorization bool   `form:"skip_secondary_authorization"`
}

// Validate validates the fields
func (f *EditOAuth2ApplicationForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// TwoFactorAuthForm for logging in with 2FA token.
type TwoFactorAuthForm struct {
	Passcode string `binding:"Required"`
}

// Validate validates the fields
func (f *TwoFactorAuthForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// TwoFactorScratchAuthForm for logging in with 2FA scratch token.
type TwoFactorScratchAuthForm struct {
	Token string `binding:"Required"`
}

// Validate validates the fields
func (f *TwoFactorScratchAuthForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// WebauthnRegistrationForm for reserving an WebAuthn name
type WebauthnRegistrationForm struct {
	Name string `binding:"Required"`
}

// Validate validates the fields
func (f *WebauthnRegistrationForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// WebauthnDeleteForm for deleting WebAuthn keys
type WebauthnDeleteForm struct {
	ID int64 `binding:"Required"`
}

// Validate validates the fields
func (f *WebauthnDeleteForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// PackageSettingForm form for package settings
type PackageSettingForm struct {
	Action string
	RepoID int64 `form:"repo_id"`
}

// Validate validates the fields
func (f *PackageSettingForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

type BlockUserForm struct {
	Action  string `binding:"Required;In(block,unblock,note)"`
	Blockee string `binding:"Required"`
	Note    string
}

func (f *BlockUserForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
