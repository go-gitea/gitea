// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"

	"github.com/Unknwon/com"
	"github.com/go-xorm/core"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/auth/ldap"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplAuths    base.TplName = "admin/auth/list"
	tplAuthNew  base.TplName = "admin/auth/new"
	tplAuthEdit base.TplName = "admin/auth/edit"
)

// Authentications show authentication config page
func Authentications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.authentication")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	var err error
	ctx.Data["Sources"], err = models.LoginSources()
	if err != nil {
		ctx.Handle(500, "LoginSources", err)
		return
	}

	ctx.Data["Total"] = models.CountLoginSources()
	ctx.HTML(200, tplAuths)
}

type dropdownItem struct {
	Name string
	Type interface{}
}

var (
	authSources = []dropdownItem{
		{models.LoginNames[models.LoginLDAP], models.LoginLDAP},
		{models.LoginNames[models.LoginDLDAP], models.LoginDLDAP},
		{models.LoginNames[models.LoginSMTP], models.LoginSMTP},
		{models.LoginNames[models.LoginPAM], models.LoginPAM},
	}
	securityProtocols = []dropdownItem{
		{models.SecurityProtocolNames[ldap.SecurityProtocolUnencrypted], ldap.SecurityProtocolUnencrypted},
		{models.SecurityProtocolNames[ldap.SecurityProtocolLDAPS], ldap.SecurityProtocolLDAPS},
		{models.SecurityProtocolNames[ldap.SecurityProtocolStartTLS], ldap.SecurityProtocolStartTLS},
	}
)

// NewAuthSource render adding a new auth source page
func NewAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["type"] = models.LoginLDAP
	ctx.Data["CurrentTypeName"] = models.LoginNames[models.LoginLDAP]
	ctx.Data["CurrentSecurityProtocol"] = models.SecurityProtocolNames[ldap.SecurityProtocolUnencrypted]
	ctx.Data["smtp_auth"] = "PLAIN"
	ctx.Data["is_active"] = true
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = models.SMTPAuths
	ctx.HTML(200, tplAuthNew)
}

func parseLDAPConfig(form auth.AuthenticationForm) *models.LDAPConfig {
	return &models.LDAPConfig{
		Source: &ldap.Source{
			Name:              form.Name,
			Host:              form.Host,
			Port:              form.Port,
			SecurityProtocol:  ldap.SecurityProtocol(form.SecurityProtocol),
			SkipVerify:        form.SkipVerify,
			BindDN:            form.BindDN,
			UserDN:            form.UserDN,
			BindPassword:      form.BindPassword,
			UserBase:          form.UserBase,
			AttributeUsername: form.AttributeUsername,
			AttributeName:     form.AttributeName,
			AttributeSurname:  form.AttributeSurname,
			AttributeMail:     form.AttributeMail,
			AttributesInBind:  form.AttributesInBind,
			Filter:            form.Filter,
			AdminFilter:       form.AdminFilter,
			Enabled:           true,
		},
	}
}

func parseSMTPConfig(form auth.AuthenticationForm) *models.SMTPConfig {
	return &models.SMTPConfig{
		Auth:           form.SMTPAuth,
		Host:           form.SMTPHost,
		Port:           form.SMTPPort,
		AllowedDomains: form.AllowedDomains,
		TLS:            form.TLS,
		SkipVerify:     form.SkipVerify,
	}
}

// NewAuthSourcePost response for adding an auth source
func NewAuthSourcePost(ctx *context.Context, form auth.AuthenticationForm) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["CurrentTypeName"] = models.LoginNames[models.LoginType(form.Type)]
	ctx.Data["CurrentSecurityProtocol"] = models.SecurityProtocolNames[ldap.SecurityProtocol(form.SecurityProtocol)]
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = models.SMTPAuths

	hasTLS := false
	var config core.Conversion
	switch models.LoginType(form.Type) {
	case models.LoginLDAP, models.LoginDLDAP:
		config = parseLDAPConfig(form)
		hasTLS = ldap.SecurityProtocol(form.SecurityProtocol) > ldap.SecurityProtocolUnencrypted
	case models.LoginSMTP:
		config = parseSMTPConfig(form)
		hasTLS = true
	case models.LoginPAM:
		config = &models.PAMConfig{
			ServiceName: form.PAMServiceName,
		}
	default:
		ctx.Error(400)
		return
	}
	ctx.Data["HasTLS"] = hasTLS

	if ctx.HasError() {
		ctx.HTML(200, tplAuthNew)
		return
	}

	if err := models.CreateLoginSource(&models.LoginSource{
		Type:      models.LoginType(form.Type),
		Name:      form.Name,
		IsActived: form.IsActive,
		Cfg:       config,
	}); err != nil {
		if models.IsErrLoginSourceAlreadyExist(err) {
			ctx.Data["Err_Name"] = true
			ctx.RenderWithErr(ctx.Tr("admin.auths.login_source_exist", err.(models.ErrLoginSourceAlreadyExist).Name), tplAuthNew, form)
		} else {
			ctx.Handle(500, "CreateSource", err)
		}
		return
	}

	log.Trace("Authentication created by admin(%s): %s", ctx.User.Name, form.Name)

	ctx.Flash.Success(ctx.Tr("admin.auths.new_success", form.Name))
	ctx.Redirect(setting.AppSubURL + "/admin/auths")
}

// EditAuthSource render editing auth source page
func EditAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = models.SMTPAuths

	source, err := models.GetLoginSourceByID(ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.Handle(500, "GetLoginSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()

	ctx.HTML(200, tplAuthEdit)
}

// EditAuthSourcePost resposne for editing auth source
func EditAuthSourcePost(ctx *context.Context, form auth.AuthenticationForm) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SMTPAuths"] = models.SMTPAuths

	source, err := models.GetLoginSourceByID(ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.Handle(500, "GetLoginSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()

	if ctx.HasError() {
		ctx.HTML(200, tplAuthEdit)
		return
	}

	var config core.Conversion
	switch models.LoginType(form.Type) {
	case models.LoginLDAP, models.LoginDLDAP:
		config = parseLDAPConfig(form)
	case models.LoginSMTP:
		config = parseSMTPConfig(form)
	case models.LoginPAM:
		config = &models.PAMConfig{
			ServiceName: form.PAMServiceName,
		}
	default:
		ctx.Error(400)
		return
	}

	source.Name = form.Name
	source.IsActived = form.IsActive
	source.Cfg = config
	if err := models.UpdateSource(source); err != nil {
		ctx.Handle(500, "UpdateSource", err)
		return
	}
	log.Trace("Authentication changed by admin(%s): %d", ctx.User.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.update_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/auths/" + com.ToStr(form.ID))
}

// DeleteAuthSource response for deleting an auth source
func DeleteAuthSource(ctx *context.Context) {
	source, err := models.GetLoginSourceByID(ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.Handle(500, "GetLoginSourceByID", err)
		return
	}

	if err = models.DeleteSource(source); err != nil {
		if models.IsErrLoginSourceInUse(err) {
			ctx.Flash.Error(ctx.Tr("admin.auths.still_in_used"))
		} else {
			ctx.Flash.Error(fmt.Sprintf("DeleteSource: %v", err))
		}
		ctx.JSON(200, map[string]interface{}{
			"redirect": setting.AppSubURL + "/admin/auths/" + ctx.Params(":authid"),
		})
		return
	}
	log.Trace("Authentication deleted by admin(%s): %d", ctx.User.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/auths",
	})
}
