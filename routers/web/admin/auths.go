// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"code.gitea.io/gitea/models/login"
	"code.gitea.io/gitea/modules/auth/pam"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/ldap"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	pamService "code.gitea.io/gitea/services/auth/source/pam"
	"code.gitea.io/gitea/services/auth/source/smtp"
	"code.gitea.io/gitea/services/auth/source/sspi"
	"code.gitea.io/gitea/services/forms"

	"xorm.io/xorm/convert"
)

const (
	tplAuths    base.TplName = "admin/auth/list"
	tplAuthNew  base.TplName = "admin/auth/new"
	tplAuthEdit base.TplName = "admin/auth/edit"
)

var (
	separatorAntiPattern = regexp.MustCompile(`[^\w-\.]`)
	langCodePattern      = regexp.MustCompile(`^[a-z]{2}-[A-Z]{2}$`)
)

// Authentications show authentication config page
func Authentications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.authentication")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	var err error
	ctx.Data["Sources"], err = login.Sources()
	if err != nil {
		ctx.ServerError("login.Sources", err)
		return
	}

	ctx.Data["Total"] = login.CountSources()
	ctx.HTML(http.StatusOK, tplAuths)
}

type dropdownItem struct {
	Name string
	Type interface{}
}

var (
	authSources = func() []dropdownItem {
		items := []dropdownItem{
			{login.LDAP.String(), login.LDAP},
			{login.DLDAP.String(), login.DLDAP},
			{login.SMTP.String(), login.SMTP},
			{login.OAuth2.String(), login.OAuth2},
			{login.SSPI.String(), login.SSPI},
		}
		if pam.Supported {
			items = append(items, dropdownItem{login.Names[login.PAM], login.PAM})
		}
		return items
	}()

	securityProtocols = []dropdownItem{
		{ldap.SecurityProtocolNames[ldap.SecurityProtocolUnencrypted], ldap.SecurityProtocolUnencrypted},
		{ldap.SecurityProtocolNames[ldap.SecurityProtocolLDAPS], ldap.SecurityProtocolLDAPS},
		{ldap.SecurityProtocolNames[ldap.SecurityProtocolStartTLS], ldap.SecurityProtocolStartTLS},
	}
)

// NewAuthSource render adding a new auth source page
func NewAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["type"] = login.LDAP
	ctx.Data["CurrentTypeName"] = login.Names[login.LDAP]
	ctx.Data["CurrentSecurityProtocol"] = ldap.SecurityProtocolNames[ldap.SecurityProtocolUnencrypted]
	ctx.Data["smtp_auth"] = "PLAIN"
	ctx.Data["is_active"] = true
	ctx.Data["is_sync_enabled"] = true
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers

	ctx.Data["SSPIAutoCreateUsers"] = true
	ctx.Data["SSPIAutoActivateUsers"] = true
	ctx.Data["SSPIStripDomainNames"] = true
	ctx.Data["SSPISeparatorReplacement"] = "_"
	ctx.Data["SSPIDefaultLanguage"] = ""

	// only the first as default
	ctx.Data["oauth2_provider"] = oauth2providers[0]

	ctx.HTML(http.StatusOK, tplAuthNew)
}

func parseLDAPConfig(form forms.AuthenticationForm) *ldap.Source {
	var pageSize uint32
	if form.UsePagedSearch {
		pageSize = uint32(form.SearchPageSize)
	}
	return &ldap.Source{
		Name:                  form.Name,
		Host:                  form.Host,
		Port:                  form.Port,
		SecurityProtocol:      ldap.SecurityProtocol(form.SecurityProtocol),
		SkipVerify:            form.SkipVerify,
		BindDN:                form.BindDN,
		UserDN:                form.UserDN,
		BindPassword:          form.BindPassword,
		UserBase:              form.UserBase,
		AttributeUsername:     form.AttributeUsername,
		AttributeName:         form.AttributeName,
		AttributeSurname:      form.AttributeSurname,
		AttributeMail:         form.AttributeMail,
		AttributesInBind:      form.AttributesInBind,
		AttributeSSHPublicKey: form.AttributeSSHPublicKey,
		AttributeAvatar:       form.AttributeAvatar,
		SearchPageSize:        pageSize,
		Filter:                form.Filter,
		GroupsEnabled:         form.GroupsEnabled,
		GroupDN:               form.GroupDN,
		GroupFilter:           form.GroupFilter,
		GroupMemberUID:        form.GroupMemberUID,
		UserUID:               form.UserUID,
		AdminFilter:           form.AdminFilter,
		RestrictedFilter:      form.RestrictedFilter,
		AllowDeactivateAll:    form.AllowDeactivateAll,
		Enabled:               true,
		SkipLocalTwoFA:        form.SkipLocalTwoFA,
	}
}

func parseSMTPConfig(form forms.AuthenticationForm) *smtp.Source {
	return &smtp.Source{
		Auth:           form.SMTPAuth,
		Host:           form.SMTPHost,
		Port:           form.SMTPPort,
		AllowedDomains: form.AllowedDomains,
		ForceSMTPS:     form.ForceSMTPS,
		SkipVerify:     form.SkipVerify,
		HeloHostname:   form.HeloHostname,
		DisableHelo:    form.DisableHelo,
		SkipLocalTwoFA: form.SkipLocalTwoFA,
	}
}

func parseOAuth2Config(form forms.AuthenticationForm) *oauth2.Source {
	var customURLMapping *oauth2.CustomURLMapping
	if form.Oauth2UseCustomURL {
		customURLMapping = &oauth2.CustomURLMapping{
			TokenURL:   form.Oauth2TokenURL,
			AuthURL:    form.Oauth2AuthURL,
			ProfileURL: form.Oauth2ProfileURL,
			EmailURL:   form.Oauth2EmailURL,
			Tenant:     form.Oauth2Tenant,
		}
	} else {
		customURLMapping = nil
	}
	return &oauth2.Source{
		Provider:                      form.Oauth2Provider,
		ClientID:                      form.Oauth2Key,
		ClientSecret:                  form.Oauth2Secret,
		OpenIDConnectAutoDiscoveryURL: form.OpenIDConnectAutoDiscoveryURL,
		CustomURLMapping:              customURLMapping,
		IconURL:                       form.Oauth2IconURL,
		SkipLocalTwoFA:                form.SkipLocalTwoFA,
	}
}

func parseSSPIConfig(ctx *context.Context, form forms.AuthenticationForm) (*sspi.Source, error) {
	if util.IsEmptyString(form.SSPISeparatorReplacement) {
		ctx.Data["Err_SSPISeparatorReplacement"] = true
		return nil, errors.New(ctx.Tr("form.SSPISeparatorReplacement") + ctx.Tr("form.require_error"))
	}
	if separatorAntiPattern.MatchString(form.SSPISeparatorReplacement) {
		ctx.Data["Err_SSPISeparatorReplacement"] = true
		return nil, errors.New(ctx.Tr("form.SSPISeparatorReplacement") + ctx.Tr("form.alpha_dash_dot_error"))
	}

	if form.SSPIDefaultLanguage != "" && !langCodePattern.MatchString(form.SSPIDefaultLanguage) {
		ctx.Data["Err_SSPIDefaultLanguage"] = true
		return nil, errors.New(ctx.Tr("form.lang_select_error"))
	}

	return &sspi.Source{
		AutoCreateUsers:      form.SSPIAutoCreateUsers,
		AutoActivateUsers:    form.SSPIAutoActivateUsers,
		StripDomainNames:     form.SSPIStripDomainNames,
		SeparatorReplacement: form.SSPISeparatorReplacement,
		DefaultLanguage:      form.SSPIDefaultLanguage,
	}, nil
}

// NewAuthSourcePost response for adding an auth source
func NewAuthSourcePost(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.AuthenticationForm)
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["CurrentTypeName"] = login.Type(form.Type).String()
	ctx.Data["CurrentSecurityProtocol"] = ldap.SecurityProtocolNames[ldap.SecurityProtocol(form.SecurityProtocol)]
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers

	ctx.Data["SSPIAutoCreateUsers"] = true
	ctx.Data["SSPIAutoActivateUsers"] = true
	ctx.Data["SSPIStripDomainNames"] = true
	ctx.Data["SSPISeparatorReplacement"] = "_"
	ctx.Data["SSPIDefaultLanguage"] = ""

	hasTLS := false
	var config convert.Conversion
	switch login.Type(form.Type) {
	case login.LDAP, login.DLDAP:
		config = parseLDAPConfig(form)
		hasTLS = ldap.SecurityProtocol(form.SecurityProtocol) > ldap.SecurityProtocolUnencrypted
	case login.SMTP:
		config = parseSMTPConfig(form)
		hasTLS = true
	case login.PAM:
		config = &pamService.Source{
			ServiceName:    form.PAMServiceName,
			EmailDomain:    form.PAMEmailDomain,
			SkipLocalTwoFA: form.SkipLocalTwoFA,
		}
	case login.OAuth2:
		config = parseOAuth2Config(form)
	case login.SSPI:
		var err error
		config, err = parseSSPIConfig(ctx, form)
		if err != nil {
			ctx.RenderWithErr(err.Error(), tplAuthNew, form)
			return
		}
		existing, err := login.SourcesByType(login.SSPI)
		if err != nil || len(existing) > 0 {
			ctx.Data["Err_Type"] = true
			ctx.RenderWithErr(ctx.Tr("admin.auths.login_source_of_type_exist"), tplAuthNew, form)
			return
		}
	default:
		ctx.Error(http.StatusBadRequest)
		return
	}
	ctx.Data["HasTLS"] = hasTLS

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplAuthNew)
		return
	}

	if err := login.CreateSource(&login.Source{
		Type:          login.Type(form.Type),
		Name:          form.Name,
		IsActive:      form.IsActive,
		IsSyncEnabled: form.IsSyncEnabled,
		Cfg:           config,
	}); err != nil {
		if login.IsErrSourceAlreadyExist(err) {
			ctx.Data["Err_Name"] = true
			ctx.RenderWithErr(ctx.Tr("admin.auths.login_source_exist", err.(login.ErrSourceAlreadyExist).Name), tplAuthNew, form)
		} else {
			ctx.ServerError("login.CreateSource", err)
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
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers

	source, err := login.GetSourceByID(ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.ServerError("login.GetSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()

	if source.IsOAuth2() {
		type Named interface {
			Name() string
		}

		for _, provider := range oauth2providers {
			if provider.Name() == source.Cfg.(Named).Name() {
				ctx.Data["CurrentOAuth2Provider"] = provider
				break
			}
		}

	}
	ctx.HTML(http.StatusOK, tplAuthEdit)
}

// EditAuthSourcePost response for editing auth source
func EditAuthSourcePost(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.AuthenticationForm)
	ctx.Data["Title"] = ctx.Tr("admin.auths.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers

	source, err := login.GetSourceByID(ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.ServerError("login.GetSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplAuthEdit)
		return
	}

	var config convert.Conversion
	switch login.Type(form.Type) {
	case login.LDAP, login.DLDAP:
		config = parseLDAPConfig(form)
	case login.SMTP:
		config = parseSMTPConfig(form)
	case login.PAM:
		config = &pamService.Source{
			ServiceName: form.PAMServiceName,
			EmailDomain: form.PAMEmailDomain,
		}
	case login.OAuth2:
		config = parseOAuth2Config(form)
	case login.SSPI:
		config, err = parseSSPIConfig(ctx, form)
		if err != nil {
			ctx.RenderWithErr(err.Error(), tplAuthEdit, form)
			return
		}
	default:
		ctx.Error(http.StatusBadRequest)
		return
	}

	source.Name = form.Name
	source.IsActive = form.IsActive
	source.IsSyncEnabled = form.IsSyncEnabled
	source.Cfg = config
	if err := login.UpdateSource(source); err != nil {
		if oauth2.IsErrOpenIDConnectInitialize(err) {
			ctx.Flash.Error(err.Error(), true)
			ctx.HTML(http.StatusOK, tplAuthEdit)
		} else {
			ctx.ServerError("UpdateSource", err)
		}
		return
	}
	log.Trace("Authentication changed by admin(%s): %d", ctx.User.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.update_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/auths/" + strconv.FormatInt(form.ID, 10))
}

// DeleteAuthSource response for deleting an auth source
func DeleteAuthSource(ctx *context.Context) {
	source, err := login.GetSourceByID(ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.ServerError("login.GetSourceByID", err)
		return
	}

	if err = auth_service.DeleteLoginSource(source); err != nil {
		if login.IsErrSourceInUse(err) {
			ctx.Flash.Error(ctx.Tr("admin.auths.still_in_used"))
		} else {
			ctx.Flash.Error(fmt.Sprintf("DeleteLoginSource: %v", err))
		}
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"redirect": setting.AppSubURL + "/admin/auths/" + url.PathEscape(ctx.Params(":authid")),
		})
		return
	}
	log.Trace("Authentication deleted by admin(%s): %d", ctx.User.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.deletion_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/auths",
	})
}
