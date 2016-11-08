// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"mime/multipart"

	"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"
)

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
	RunUser      string `binding:"Required"`
	Domain       string `binding:"Required"`
	SSHPort      int
	HTTPPort     string `binding:"Required"`
	AppUrl       string `binding:"Required"`
	LogRootPath  string `binding:"Required"`

	SMTPHost        string
	SMTPFrom        string
	SMTPEmail       string `binding:"OmitEmpty;Email;MaxSize(254)" locale:"install.mailer_user"`
	SMTPPasswd      string
	RegisterConfirm bool
	MailNotify      bool

	OfflineMode           bool
	DisableGravatar       bool
	EnableFederatedAvatar bool
	DisableRegistration   bool
	EnableCaptcha         bool
	RequireSignInView     bool

	AdminName          string `binding:"OmitEmpty;AlphaDashDot;MaxSize(30)" locale:"install.admin_name"`
	AdminPasswd        string `binding:"OmitEmpty;MaxSize(255)" locale:"install.admin_password"`
	AdminConfirmPasswd string
	AdminEmail         string `binding:"OmitEmpty;MinSize(3);MaxSize(254);Include(@)" locale:"install.admin_email"`
}

func (f *InstallForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

//    _____   ____ _________________ ___
//   /  _  \ |    |   \__    ___/   |   \
//  /  /_\  \|    |   / |    | /    ~    \
// /    |    \    |  /  |    | \    Y    /
// \____|__  /______/   |____|  \___|_  /
//         \/                         \/

type RegisterForm struct {
	UserName string `binding:"Required;AlphaDashDot;MaxSize(35)"`
	Email    string `binding:"Required;Email;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
	Retype   string
}

func (f *RegisterForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type SignInForm struct {
	UserName string `binding:"Required;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
	Remember bool
}

func (f *SignInForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

//   __________________________________________.___ _______    ________  _________
//  /   _____/\_   _____/\__    ___/\__    ___/|   |\      \  /  _____/ /   _____/
//  \_____  \  |    __)_   |    |     |    |   |   |/   |   \/   \  ___ \_____  \
//  /        \ |        \  |    |     |    |   |   /    |    \    \_\  \/        \
// /_______  //_______  /  |____|     |____|   |___\____|__  /\______  /_______  /
//         \/         \/                                   \/        \/        \/

type UpdateProfileForm struct {
	Name     string `binding:"OmitEmpty;MaxSize(35)"`
	FullName string `binding:"MaxSize(100)"`
	Email    string `binding:"Required;Email;MaxSize(254)"`
	Website  string `binding:"Url;MaxSize(100)"`
	Location string `binding:"MaxSize(50)"`
}

func (f *UpdateProfileForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

const (
	AvatarLocal  string = "local"
	AvatarByMail string = "bymail"
)

type AvatarForm struct {
	Source      string
	Avatar      *multipart.FileHeader
	Gravatar    string `binding:"OmitEmpty;Email;MaxSize(254)"`
	Federavatar bool
}

func (f *AvatarForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type AddEmailForm struct {
	Email string `binding:"Required;Email;MaxSize(254)"`
}

func (f *AddEmailForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type ChangePasswordForm struct {
	OldPassword string `form:"old_password" binding:"Required;MinSize(1);MaxSize(255)"`
	Password    string `form:"password" binding:"Required;MaxSize(255)"`
	Retype      string `form:"retype"`
}

func (f *ChangePasswordForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type AddSSHKeyForm struct {
	Title   string `binding:"Required;MaxSize(50)"`
	Content string `binding:"Required"`
}

func (f *AddSSHKeyForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type NewAccessTokenForm struct {
	Name string `binding:"Required"`
}

func (f *NewAccessTokenForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}
