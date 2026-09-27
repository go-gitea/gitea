// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	auth_service "gitea.dev/services/auth"
	"gitea.dev/services/auth/source/oauth2"
	"gitea.dev/services/auth/source/smtp"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplAuths    templates.TplName = "admin/auth/list"
	tplAuthNew  templates.TplName = "admin/auth/new"
	tplAuthEdit templates.TplName = "admin/auth/edit"
)

var ()

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
			{auth.SMTP.String(), auth.SMTP},
			{auth.OAuth2.String(), auth.OAuth2},
		}
		return items
	}()
)

// NewAuthSource render adding a new auth source page
func NewAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["type"] = auth.SMTP.Int()
	ctx.Data["CurrentTypeName"] = auth.Names[auth.SMTP]
	ctx.Data["smtp_auth"] = "PLAIN"
	ctx.Data["is_active"] = true
	ctx.Data["is_sync_enabled"] = true
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers(ctx)
	ctx.Data["OAuth2Providers"] = oauth2providers

	// only the first as default
	if len(oauth2providers) > 0 {
		ctx.Data["oauth2_provider"] = oauth2providers[0].Name()
	}

	ctx.HTML(http.StatusOK, tplAuthNew)
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
	for s := range strings.SplitSeq(form.Oauth2Scopes, ",") {
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
		GroupClaimName:                form.Oauth2GroupClaimName,
		RestrictedGroup:               form.Oauth2RestrictedGroup,
		AdminGroup:                    form.Oauth2AdminGroup,
		GroupTeamMap:                  form.Oauth2GroupTeamMap,
		GroupTeamMapRemoval:           form.Oauth2GroupTeamMapRemoval,

		SSHPublicKeyClaimName: form.Oauth2SSHPublicKeyClaimName,
		FullNameClaimName:     form.Oauth2FullNameClaimName,
		ExternalIDClaim:       form.OpenIDConnectExternalIDClaim,
	}
}

// NewAuthSourcePost response for adding an auth source
func NewAuthSourcePost(ctx *context.Context) {
	form := *web.GetForm[*forms.AuthenticationForm](ctx)
	ctx.Data["Title"] = ctx.Tr("admin.auths.new")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["CurrentTypeName"] = auth.Type(form.Type).String()
	ctx.Data["AuthSources"] = authSources
	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers(ctx)
	ctx.Data["OAuth2Providers"] = oauth2providers

	hasTLS := false
	var config auth.Config
	switch auth.Type(form.Type) {
	case auth.SMTP:
		config = parseSMTPConfig(form)
		hasTLS = true
	case auth.OAuth2:
		oauth2Config := parseOAuth2Config(form)
		config = oauth2Config
		if oauth2Config.Provider == "openidConnect" {
			discoveryURL, err := url.Parse(oauth2Config.OpenIDConnectAutoDiscoveryURL)
			if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
				ctx.Data["Err_DiscoveryURL"] = true
				ctx.RenderWithErrDeprecated(ctx.Tr("admin.auths.invalid_openIdConnectAutoDiscoveryURL"), tplAuthNew, form)
				return
			}
		}
	default:
		ctx.HTTPError(http.StatusBadRequest)
		return
	}
	ctx.Data["HasTLS"] = hasTLS

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplAuthNew)
		return
	}

	if err := auth_service.CreateSource(ctx, &auth.Source{
		Type:            auth.Type(form.Type),
		Name:            form.Name,
		IsActive:        form.IsActive,
		IsSyncEnabled:   form.IsSyncEnabled,
		TwoFactorPolicy: form.TwoFactorPolicy,
		Cfg:             config,
	}); err != nil {
		if errExist, ok := errors.AsType[auth.ErrSourceAlreadyExist](err); ok {
			ctx.Data["Err_Name"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("admin.auths.login_source_exist", errExist.Name), tplAuthNew, form)
		} else if errInit, ok := err.(oauth2.ErrOpenIDConnectInitialize); ok {
			ctx.Data["Err_DiscoveryURL"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("admin.auths.unable_to_initialize_openid", errInit.Unwrap()), tplAuthNew, form)
		} else {
			ctx.ServerError("auth.CreateSource", err)
		}
		return
	}

	log.Trace("Authentication created by admin(%s): %s", ctx.Doer.Name, form.Name)

	ctx.Flash.Success(ctx.Tr("admin.auths.new_success", form.Name))
	ctx.Redirect(setting.AppSubURL + "/-/admin/auths")
}

// EditAuthSource render editing auth source page
func EditAuthSource(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.auths.edit")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers(ctx)
	ctx.Data["OAuth2Providers"] = oauth2providers

	source, err := auth.GetSourceByID(ctx, ctx.PathParamInt64("authid"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()

	if source.IsOAuth2() {
		oauth2Source := auth.MustSourceCfg[*oauth2.Source](source)
		for _, provider := range oauth2providers {
			if provider.Name() == oauth2Source.Name() {
				ctx.Data["CurrentOAuth2Provider"] = provider
				break
			}
		}
	}

	ctx.HTML(http.StatusOK, tplAuthEdit)
}

// EditAuthSourcePost response for editing auth source
func EditAuthSourcePost(ctx *context.Context) {
	form := *web.GetForm[*forms.AuthenticationForm](ctx)
	ctx.Data["Title"] = ctx.Tr("admin.auths.edit")
	ctx.Data["PageIsAdminAuthentications"] = true

	ctx.Data["SMTPAuths"] = smtp.Authenticators
	oauth2providers := oauth2.GetSupportedOAuth2Providers(ctx)
	ctx.Data["OAuth2Providers"] = oauth2providers

	source, err := auth.GetSourceByID(ctx, ctx.PathParamInt64("authid"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}
	ctx.Data["Source"] = source
	ctx.Data["HasTLS"] = source.HasTLS()

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplAuthEdit)
		return
	}

	var config auth.Config
	switch auth.Type(form.Type) {
	case auth.SMTP:
		config = parseSMTPConfig(form)
	case auth.OAuth2:
		oauth2Config := parseOAuth2Config(form)
		config = oauth2Config
		if oauth2Config.Provider == "openidConnect" {
			discoveryURL, err := url.Parse(oauth2Config.OpenIDConnectAutoDiscoveryURL)
			if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
				ctx.Data["Err_DiscoveryURL"] = true
				ctx.RenderWithErrDeprecated(ctx.Tr("admin.auths.invalid_openIdConnectAutoDiscoveryURL"), tplAuthEdit, form)
				return
			}
		}
	default:
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	source.Name = form.Name
	source.IsActive = form.IsActive
	source.IsSyncEnabled = form.IsSyncEnabled
	source.Cfg = config
	source.TwoFactorPolicy = form.TwoFactorPolicy
	if err := auth_service.UpdateSource(ctx, source); err != nil {
		if errExist, ok := errors.AsType[auth.ErrSourceAlreadyExist](err); ok {
			ctx.Data["Err_Name"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("admin.auths.login_source_exist", errExist.Name), tplAuthEdit, form)
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
	ctx.Redirect(setting.AppSubURL + "/-/admin/auths/" + strconv.FormatInt(source.ID, 10))
}

// DeleteAuthSource response for deleting an auth source
func DeleteAuthSource(ctx *context.Context) {
	source, err := auth.GetSourceByID(ctx, ctx.PathParamInt64("authid"))
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
		ctx.JSONRedirect(setting.AppSubURL + "/-/admin/auths/" + url.PathEscape(ctx.PathParam("authid")))
		return
	}
	log.Trace("Authentication deleted by admin(%s): %d", ctx.Doer.Name, source.ID)

	ctx.Flash.Success(ctx.Tr("admin.auths.deletion_success"))
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/auths")
}
