// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"mime/multipart"

	"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"
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

	AppName      string `binding:"Required" locale:"install.app_name"`
	RepoRootPath string `binding:"Required"`
	LFSRootPath  string
	RunUser      string `binding:"Required"`
	Domain       string `binding:"Required"`
	SSHPort      int
	HTTPPort     string `binding:"Required"`
	AppURL       string `binding:"Required"`
	LogRootPath  string `binding:"Required"`

	SMTPHost        string
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
	EnableCaptcha                  bool
	RequireSignInView              bool
	DefaultKeepEmailPrivate        bool
	DefaultAllowCreateOrganization bool
	DefaultEnableTimetracking      bool
	NoReplyAddress                 string

	AdminName          string `binding:"OmitEmpty;AlphaDashDot;MaxSize(30)" locale:"install.admin_name"`
	AdminPasswd        string `binding:"OmitEmpty;MaxSize(255)" locale:"install.admin_password"`
	AdminConfirmPasswd string
	AdminEmail         string `binding:"OmitEmpty;MinSize(3);MaxSize(254);Include(@)" locale:"install.admin_email"`
}

// Validate validates the fields
func (f *InstallForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

//    _____   ____ _________________ ___
//   /  _  \ |    |   \__    ___/   |   \
//  /  /_\  \|    |   / |    | /    ~    \
// /    |    \    |  /  |    | \    Y    /
// \____|__  /______/   |____|  \___|_  /
//         \/                         \/

// RegisterForm form for registering
type RegisterForm struct {
	UserName string `binding:"Required;AlphaDashDot;MaxSize(35)"`
	Email    string `binding:"Required;Email;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
	Retype   string
}

// Validate valideates the fields
func (f *RegisterForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// SignInForm form for signing in with user/password
type SignInForm struct {
	UserName string `binding:"Required;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
	Remember bool
}

// Validate valideates the fields
func (f *SignInForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

//   __________________________________________.___ _______    ________  _________
//  /   _____/\_   _____/\__    ___/\__    ___/|   |\      \  /  _____/ /   _____/
//  \_____  \  |    __)_   |    |     |    |   |   |/   |   \/   \  ___ \_____  \
//  /        \ |        \  |    |     |    |   |   /    |    \    \_\  \/        \
// /_______  //_______  /  |____|     |____|   |___\____|__  /\______  /_______  /
//         \/         \/                                   \/        \/        \/

// UpdateProfileForm form for updating profile
type UpdateProfileForm struct {
	Name             string `binding:"AlphaDashDot;MaxSize(35)"`
	FullName         string `binding:"MaxSize(100)"`
	Email            string `binding:"Required;Email;MaxSize(254)"`
	KeepEmailPrivate bool
	Website          string `binding:"ValidUrl;MaxSize(255)"`
	Location         string `binding:"MaxSize(50)"`
}

// Validate validates the fields
func (f *UpdateProfileForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
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
func (f *AvatarForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// AddEmailForm form for adding new email
type AddEmailForm struct {
	Email string `binding:"Required;Email;MaxSize(254)"`
}

// Validate validates the fields
func (f *AddEmailForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// ChangePasswordForm form for changing password
type ChangePasswordForm struct {
	OldPassword string `form:"old_password" binding:"MaxSize(255)"`
	Password    string `form:"password" binding:"Required;MaxSize(255)"`
	Retype      string `form:"retype"`
}

// Validate validates the fields
func (f *ChangePasswordForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// AddOpenIDForm is for changing openid uri
type AddOpenIDForm struct {
	Openid string `binding:"Required;MaxSize(256)"`
}

// Validate validates the fields
func (f *AddOpenIDForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// AddKeyForm form for adding SSH/GPG key
type AddKeyForm struct {
	Type    string `binding:"OmitEmpty"`
	Title   string `binding:"Required;MaxSize(50)"`
	Content string `binding:"Required"`
}

// Validate validates the fields
func (f *AddKeyForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// NewAccessTokenForm form for creating access token
type NewAccessTokenForm struct {
	Name string `binding:"Required"`
}

// Validate valideates the fields
func (f *NewAccessTokenForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// TwoFactorAuthForm for logging in with 2FA token.
type TwoFactorAuthForm struct {
	Passcode string `binding:"Required"`
}

// Validate validates the fields
func (f *TwoFactorAuthForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// TwoFactorScratchAuthForm for logging in with 2FA scratch token.
type TwoFactorScratchAuthForm struct {
	Token string `binding:"Required"`
}

// Validate valideates the fields
func (f *TwoFactorScratchAuthForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}
