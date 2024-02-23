// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
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
	pam_service "code.gitea.io/gitea/services/auth/source/pam"
	"code.gitea.io/gitea/services/auth/source/saml"
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
	ctx.Data["PageIsAdminAuthentications"] = true

	var err error
	ctx.Data["Sources"], ctx.Data["Total"], err = db.FindAndCount[auth.Source](ctx, auth.FindSourcesOptions{})
	if err != nil {
		ctx.ServerError("auth.Sources", err)
		return
	}

	ctx.HTML(http.StatusOK, tplAuths)
}

type dropdownItem struct {
	Name string
	Type any
}

var (
	authSources = func() []dropdownItem {
		items := []dropdownItem{
			{auth.LDAP.String(), auth.LDAP},
			{auth.DLDAP.String(), auth.DLDAP},
			{auth.SMTP.String(), auth.SMTP},
			{auth.OAuth2.String(), auth.OAuth2},
			{auth.SSPI.String(), auth.SSPI},
			{auth.SAML.String(), auth.SAML},
		}
		if pam.Supported {
			items = append(items, dropdownItem{auth.Names[auth.PAM], auth.PAM})
		}
		return items
	}()

	securityProtocols = []dropdownItem{
		{ldap.SecurityProtocolNames[ldap.SecurityProtocolUnencrypted], ldap.SecurityProtocolUnencrypted},
		{ldap.SecurityProtocolNames[ldap.SecurityProtocolLDAPS], ldap.SecurityProtocolLDAPS},
		{ldap.SecurityProtocolNames[ldap.SecurityProtocolStartTLS], ldap.SecurityProtocolStartTLS},
	}

	nameIDFormats = []dropdownItem{
		{saml.NameIDFormatNames[saml.SAML20Persistent], saml.SAML20Persistent}, // use this as default value
		{saml.NameIDFormatNames[saml.SAML11Email], saml.SAML11Email},
		{saml.NameIDFormatNames[saml.SAML11Persistent], saml.SAML11Persistent},
		{saml.NameIDFormatNames[saml.SAML11Unspecified], saml.SAML11Unspecified},
		{saml.NameIDFormatNames[saml.SAML20Email], saml.SAML20Email},
		{saml.NameIDFormatNames[saml.SAML20Transient], saml.SAML20Transient},
		{saml.NameIDFormatNames[saml.SAML20Unspecified], saml.SAML20Unspecified},
	}
)

// NewAuthSource render adding a new auth source page
func NewAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["type"] = auth.LDAP.Int()
	ctx.Data["CurrentTypeName"] = auth.Names[auth.LDAP]
	ctx.Data["CurrentSecurityProtocol"] = ldap.SecurityProtocolNames[ldap.SecurityProtocolUnencrypted]
	ctx.Data["smtp_auth"] = "PLAIN"
	ctx.Data["is_active"] = true
	ctx.Data["is_sync_enabled"] = true
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["CurrentNameIDFormat"] = saml.NameIDFormatNames[saml.SAML20Persistent]
	ctx.Data["NameIDFormats"] = nameIDFormats
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers

	ctx.Data["SSPIAutoCreateUsers"] = true
	ctx.Data["SSPIAutoActivateUsers"] = true
	ctx.Data["SSPIStripDomainNames"] = true
	ctx.Data["SSPISeparatorReplacement"] = "_"
	ctx.Data["SSPIDefaultLanguage"] = ""

	// only the first as default
	ctx.Data["oauth2_provider"] = oauth2providers[0].Name()

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
		GroupTeamMap:          form.GroupTeamMap,
		GroupTeamMapRemoval:   form.GroupTeamMapRemoval,
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
	var scopes []string
	for _, s := range strings.Split(form.Oauth2Scopes, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes = append(scopes, s)
		}
	}

	return &oauth2.Source{
		Provider:                      form.Oauth2Provider,
		ClientID:                      form.Oauth2Key,
		ClientSecret:                  form.Oauth2Secret,
		OpenIDConnectAutoDiscoveryURL: form.OpenIDConnectAutoDiscoveryURL,
		CustomURLMapping:              customURLMapping,
		IconURL:                       form.Oauth2IconURL,
		Scopes:                        scopes,
		RequiredClaimName:             form.Oauth2RequiredClaimName,
		RequiredClaimValue:            form.Oauth2RequiredClaimValue,
		SkipLocalTwoFA:                form.SkipLocalTwoFA,
		GroupClaimName:                form.Oauth2GroupClaimName,
		RestrictedGroup:               form.Oauth2RestrictedGroup,
		AdminGroup:                    form.Oauth2AdminGroup,
		GroupTeamMap:                  form.Oauth2GroupTeamMap,
		GroupTeamMapRemoval:           form.Oauth2GroupTeamMapRemoval,
	}
}

func parseSSPIConfig(ctx *context.Context, form forms.AuthenticationForm) (*sspi.Source, error) {
	if util.IsEmptyString(form.SSPISeparatorReplacement) {
		ctx.Data["Err_SSPISeparatorReplacement"] = true
		return nil, errors.New(ctx.Locale.TrString("form.SSPISeparatorReplacement") + ctx.Locale.TrString("form.require_error"))
	}
	if separatorAntiPattern.MatchString(form.SSPISeparatorReplacement) {
		ctx.Data["Err_SSPISeparatorReplacement"] = true
		return nil, errors.New(ctx.Locale.TrString("form.SSPISeparatorReplacement") + ctx.Locale.TrString("form.alpha_dash_dot_error"))
	}

	if form.SSPIDefaultLanguage != "" && !langCodePattern.MatchString(form.SSPIDefaultLanguage) {
		ctx.Data["Err_SSPIDefaultLanguage"] = true
		return nil, errors.New(ctx.Locale.TrString("form.lang_select_error"))
	}

	return &sspi.Source{
		AutoCreateUsers:      form.SSPIAutoCreateUsers,
		AutoActivateUsers:    form.SSPIAutoActivateUsers,
		StripDomainNames:     form.SSPIStripDomainNames,
		SeparatorReplacement: form.SSPISeparatorReplacement,
		DefaultLanguage:      form.SSPIDefaultLanguage,
	}, nil
}

func parseSAMLConfig(ctx *context.Context, form forms.AuthenticationForm) (*saml.Source, error) {
	if util.IsEmptyString(form.IdentityProviderMetadata) && util.IsEmptyString(form.IdentityProviderMetadataURL) {
		return nil, fmt.Errorf("%s %s", ctx.Tr("form.SAMLMetadata"), ctx.Tr("form.require_error"))
	}

	if !util.IsEmptyString(form.IdentityProviderMetadataURL) {
		_, err := url.Parse(form.IdentityProviderMetadataURL)
		if err != nil {
			return nil, fmt.Errorf("%s", ctx.Tr("form.SAMLMetadataURL"))
		}
	}

	// check the integrity of the certificate and private key (autogenerated if these form fields are blank)
	if !util.IsEmptyString(form.ServiceProviderCertificate) && !util.IsEmptyString(form.ServiceProviderPrivateKey) {
		keyPair, err := tls.X509KeyPair([]byte(form.ServiceProviderCertificate), []byte(form.ServiceProviderPrivateKey))
		if err != nil {
			return nil, err
		}
		keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
		if err != nil {
			return nil, err
		}
	} else {
		privateKey, cert, err := saml.GenerateSAMLSPKeypair()
		if err != nil {
			return nil, err
		}

		form.ServiceProviderPrivateKey = privateKey
		form.ServiceProviderCertificate = cert
	}

	return &saml.Source{
		IdentityProviderMetadata:                 form.IdentityProviderMetadata,
		IdentityProviderMetadataURL:              form.IdentityProviderMetadataURL,
		InsecureSkipAssertionSignatureValidation: form.InsecureSkipAssertionSignatureValidation,
		NameIDFormat:                             saml.NameIDFormat(form.NameIDFormat),
		ServiceProviderCertificate:               form.ServiceProviderCertificate,
		ServiceProviderPrivateKey:                form.ServiceProviderPrivateKey,
		EmailAssertionKey:                        form.EmailAssertionKey,
		NameAssertionKey:                         form.NameAssertionKey,
		UsernameAssertionKey:                     form.UsernameAssertionKey,
		IconURL:                                  form.SAMLIconURL,
	}, nil
}

// NewAuthSourcePost response for adding an auth source
func NewAuthSourcePost(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.AuthenticationForm)
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["CurrentTypeName"] = auth.Type(form.Type).String()
	ctx.Data["CurrentSecurityProtocol"] = ldap.SecurityProtocolNames[ldap.SecurityProtocol(form.SecurityProtocol)]
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers
	ctx.Data["CurrentNameIDFormat"] = saml.NameIDFormatNames[saml.NameIDFormat(form.NameIDFormat)]
	ctx.Data["NameIDFormats"] = nameIDFormats

	ctx.Data["SSPIAutoCreateUsers"] = true
	ctx.Data["SSPIAutoActivateUsers"] = true
	ctx.Data["SSPIStripDomainNames"] = true
	ctx.Data["SSPISeparatorReplacement"] = "_"
	ctx.Data["SSPIDefaultLanguage"] = ""

	hasTLS := false
	var config convert.Conversion
	switch auth.Type(form.Type) {
	case auth.LDAP, auth.DLDAP:
		config = parseLDAPConfig(form)
		hasTLS = ldap.SecurityProtocol(form.SecurityProtocol) > ldap.SecurityProtocolUnencrypted
	case auth.SMTP:
		config = parseSMTPConfig(form)
		hasTLS = true
	case auth.PAM:
		config = &pam_service.Source{
			ServiceName:    form.PAMServiceName,
			EmailDomain:    form.PAMEmailDomain,
			SkipLocalTwoFA: form.SkipLocalTwoFA,
		}
	case auth.OAuth2:
		config = parseOAuth2Config(form)
		oauth2Config := config.(*oauth2.Source)
		if oauth2Config.Provider == "openidConnect" {
			discoveryURL, err := url.Parse(oauth2Config.OpenIDConnectAutoDiscoveryURL)
			if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
				ctx.Data["Err_DiscoveryURL"] = true
				ctx.RenderWithErr(ctx.Tr("admin.auths.invalid_openIdConnectAutoDiscoveryURL"), tplAuthNew, form)
				return
			}
		}
	case auth.SSPI:
		var err error
		config, err = parseSSPIConfig(ctx, form)
		if err != nil {
			ctx.RenderWithErr(err.Error(), tplAuthNew, form)
			return
		}
		existing, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{LoginType: auth.SSPI})
		if err != nil || len(existing) > 0 {
			ctx.Data["Err_Type"] = true
			ctx.RenderWithErr(ctx.Tr("admin.auths.login_source_of_type_exist"), tplAuthNew, form)
			return
		}
	case auth.SAML:
		var err error
		config, err = parseSAMLConfig(ctx, form)
		if err != nil {
			ctx.RenderWithErr(err.Error(), tplAuthNew, form)
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

	if err := auth.CreateSource(ctx, &auth.Source{
		Type:          auth.Type(form.Type),
		Name:          form.Name,
		IsActive:      form.IsActive,
		IsSyncEnabled: form.IsSyncEnabled,
		Cfg:           config,
	}); err != nil {
		if auth.IsErrSourceAlreadyExist(err) {
			ctx.Data["Err_Name"] = true
			ctx.RenderWithErr(ctx.Tr("admin.auths.login_source_exist", err.(auth.ErrSourceAlreadyExist).Name), tplAuthNew, form)
		} else if oauth2.IsErrOpenIDConnectInitialize(err) {
			ctx.Data["Err_DiscoveryURL"] = true
			unwrapped := err.(oauth2.ErrOpenIDConnectInitialize).Unwrap()
			ctx.RenderWithErr(ctx.Tr("admin.auths.unable_to_initialize_openid", unwrapped), tplAuthNew, form)
		} else {
			ctx.ServerError("auth.CreateSource", err)
		}
		return
	}

	log.Trace("Authentication created by admin(%s): %s", ctx.Doer.Name, form.Name)

	ctx.Flash.Success(ctx.Tr("admin.auths.new_success", form.Name))
	ctx.Redirect(setting.AppSubURL + "/admin/auths")
}

// EditAuthSource render editing auth source page
func EditAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.edit")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SecurityProtocols"] = securityProtocols
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers
	ctx.Data["NameIDFormats"] = nameIDFormats

	source, err := auth.GetSourceByID(ctx, ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()
	if source.IsSAML() {
		ctx.Data["CurrentNameIDFormat"] = saml.NameIDFormatNames[source.Cfg.(*saml.Source).NameIDFormat]
	}

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
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers()
	ctx.Data["OAuth2Providers"] = oauth2providers

	source, err := auth.GetSourceByID(ctx, ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()
	ctx.Data["CurrentNameIDFormat"] = saml.NameIDFormatNames[saml.SAML20Persistent]
	ctx.Data["NameIDFormats"] = nameIDFormats

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplAuthEdit)
		return
	}

	var config convert.Conversion
	switch auth.Type(form.Type) {
	case auth.LDAP, auth.DLDAP:
		config = parseLDAPConfig(form)
	case auth.SMTP:
		config = parseSMTPConfig(form)
	case auth.PAM:
		config = &pam_service.Source{
			ServiceName: form.PAMServiceName,
			EmailDomain: form.PAMEmailDomain,
		}
	case auth.OAuth2:
		config = parseOAuth2Config(form)
		oauth2Config := config.(*oauth2.Source)
		if oauth2Config.Provider == "openidConnect" {
			discoveryURL, err := url.Parse(oauth2Config.OpenIDConnectAutoDiscoveryURL)
			if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
				ctx.Data["Err_DiscoveryURL"] = true
				ctx.RenderWithErr(ctx.Tr("admin.auths.invalid_openIdConnectAutoDiscoveryURL"), tplAuthEdit, form)
				return
			}
		}
	case auth.SSPI:
		config, err = parseSSPIConfig(ctx, form)
		if err != nil {
			ctx.RenderWithErr(err.Error(), tplAuthEdit, form)
			return
		}
	case auth.SAML:
		config, err = parseSAMLConfig(ctx, form)
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
	if err := auth.UpdateSource(ctx, source); err != nil {
		if auth.IsErrSourceAlreadyExist(err) {
			ctx.Data["Err_Name"] = true
			ctx.RenderWithErr(ctx.Tr("admin.auths.login_source_exist", err.(auth.ErrSourceAlreadyExist).Name), tplAuthEdit, form)
		} else if oauth2.IsErrOpenIDConnectInitialize(err) {
			ctx.Flash.Error(err.Error(), true)
			ctx.Data["Err_DiscoveryURL"] = true
			ctx.HTML(http.StatusOK, tplAuthEdit)
		} else {
			ctx.ServerError("UpdateSource", err)
		}
		return
	}
	log.Trace("Authentication changed by admin(%s): %d", ctx.Doer.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.update_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/auths/" + strconv.FormatInt(form.ID, 10))
}

// DeleteAuthSource response for deleting an auth source
func DeleteAuthSource(ctx *context.Context) {
	source, err := auth.GetSourceByID(ctx, ctx.ParamsInt64(":authid"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}

	if err = auth_service.DeleteSource(ctx, source); err != nil {
		if auth.IsErrSourceInUse(err) {
			ctx.Flash.Error(ctx.Tr("admin.auths.still_in_used"))
		} else {
			ctx.Flash.Error(fmt.Sprintf("auth_service.DeleteSource: %v", err))
		}
		ctx.JSONRedirect(setting.AppSubURL + "/admin/auths/" + url.PathEscape(ctx.Params(":authid")))
		return
	}
	log.Trace("Authentication deleted by admin(%s): %d", ctx.Doer.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.deletion_success"))
	ctx.JSONRedirect(setting.AppSubURL + "/admin/auths")
}
