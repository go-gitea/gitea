// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	stdctx "context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	"gitea.dev/services/oauth2_provider"
)

const (
	tplDeviceAuthorization         templates.TplName = "user/auth/device"
	tplDeviceAuthorizationComplete templates.TplName = "user/auth/device_complete"
	oauthDeviceAuthorizationIDKey                    = "device_authorization_id"
)

// loadPublicOAuth2App loads the client and rejects it unless it is a public client,
// which is the only client type RFC 8628 allows to run the device flow.
func loadPublicOAuth2App(ctx *context.Context, clientID string) *auth_model.OAuth2Application {
	app, err := auth_model.GetOAuth2ApplicationByClientID(ctx, clientID)
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: %q", clientID),
		})
		return nil
	}
	if app.ConfidentialClient {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "device authorization is only supported for public clients",
		})
		return nil
	}
	return app
}

// DeviceAuthorizationOAuth issues a device code to a public OAuth client.
func DeviceAuthorizationOAuth(ctx *context.Context) {
	form := web.GetForm[*forms.DeviceAuthorizationForm](ctx)
	app := loadPublicOAuth2App(ctx, form.ClientID)
	if app == nil {
		return
	}

	deviceAuthorization, deviceCode, err := auth_model.CreateOAuth2DeviceAuthorization(ctx, app, form.Scope, ctx.RemoteAddr())
	if err != nil {
		if errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationLimitReached) {
			handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
				ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "too many pending device authorizations for this client",
			})
			return
		}
		handleDeviceAccessTokenServerError(ctx, "CreateOAuth2DeviceAuthorization", err)
		return
	}

	verificationURI := strings.TrimSuffix(setting.AppURL, "/") + "/login/oauth/device"
	formattedUserCode := deviceAuthorization.FormattedUserCode()

	ctx.JSON(http.StatusOK, oauth2_provider.DeviceAuthorizationResponse{
		DeviceCode:              deviceCode,
		UserCode:                formattedUserCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + url.QueryEscape(formattedUserCode),
		ExpiresIn:               max(int64(deviceAuthorization.ExpiresAtUnix-timeutil.TimeStampNow()), 0),
		Interval:                deviceAuthorization.PollIntervalSeconds,
	})
}

// DeviceVerifyShowOAuth renders the device verification entry form (GET).
// When a user_code query parameter is present, the form is pre-filled so the
// user can submit it as a POST, which is where session state is written.
func DeviceVerifyShowOAuth(ctx *context.Context) {
	form := web.GetForm[*forms.DeviceVerificationForm](ctx)
	renderOAuthDeviceAuthorizationEntry(ctx, form.UserCode)
}

// DeviceVerifyOAuth validates the submitted device code and hands the browser to
// the consent page. It answers JSON because the entry form is a "form-fetch-action".
func DeviceVerifyOAuth(ctx *context.Context) {
	if ctx.DoerNeedTwoFactorAuth() {
		ctx.JSONError(ctx.Tr("auth.device_two_factor_required"))
		return
	}

	form := web.GetForm[*forms.DeviceVerificationForm](ctx)
	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByUserCode(ctx, form.UserCode)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.ServerError("GetOAuth2DeviceAuthorizationByUserCode", err)
		return
	}
	if err != nil || deviceAuthorization.IsExpired() {
		ctx.JSONError(ctx.Tr("auth.device_code_invalid"))
		return
	}
	if deviceAuthorization.Status == auth_model.OAuth2DeviceAuthorizationDenied {
		ctx.JSONError(ctx.Tr("auth.device_code_denied"))
		return
	}

	if err := ctx.Session.Set(oauthDeviceAuthorizationIDKey, strconv.FormatInt(deviceAuthorization.ID, 10)); err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Unable to save changes to the session: %v", err)
	}
	ctx.JSONRedirect(setting.AppSubURL + "/login/oauth/device/authorize")
}

// DeviceAuthorizeShowOAuth renders the consent screen for the device code held in the session.
func DeviceAuthorizeShowOAuth(ctx *context.Context) {
	if !oauthDoerAuthorizePreCheck(ctx, "") {
		return
	}
	deviceAuthorizationID, _ := strconv.ParseInt(fmt.Sprint(ctx.Session.Get(oauthDeviceAuthorizationIDKey)), 10, 64)
	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(ctx, deviceAuthorizationID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.ServerError("GetOAuth2DeviceAuthorizationByID", err)
		return
	}
	if err != nil || deviceAuthorization.IsExpired() {
		renderOAuthDeviceAuthorizationError(ctx, "auth.device_code_invalid")
		return
	}

	switch deviceAuthorization.Status {
	case auth_model.OAuth2DeviceAuthorizationDenied:
		renderOAuthDeviceAuthorizationComplete(ctx, false)
		return
	case auth_model.OAuth2DeviceAuthorizationConsumed, auth_model.OAuth2DeviceAuthorizationApproved:
		renderOAuthDeviceAuthorizationComplete(ctx, true)
		return
	}

	app, err := auth_model.GetOAuth2ApplicationByID(ctx, deviceAuthorization.ApplicationID)
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}

	grant, err := app.GetGrantByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetGrantByUserID", err)
		return
	}
	// consent could never succeed against a mismatched grant, so deny now instead of
	// leaving the device polling "authorization_pending" until it expires
	if grant != nil && grant.Scope != deviceAuthorization.Scope {
		if err := deviceAuthorization.MarkDenied(ctx, ctx.Doer.ID); err != nil {
			handleDeviceAuthorizationWriteError(ctx, "MarkDenied", deviceAuthorization.ID, err)
			return
		}
		renderOAuthDeviceAuthorizationError(ctx, "auth.device_scope_mismatch")
		return
	}

	// SkipSecondaryAuthorization is deliberately not honoured here: the device requesting
	// the code may not be the user's, so RFC 8628 §5.4 wants the app and scopes on screen
	if err := setOAuthDeviceAuthorizationData(ctx, app, deviceAuthorization); err != nil {
		ctx.ServerError("setOAuthDeviceAuthorizationData", err)
		return
	}
	ctx.HTML(http.StatusOK, tplDeviceAuthorization)
}

// DeviceGrantApplicationOAuth stores the user's device-flow consent decision.
func DeviceGrantApplicationOAuth(ctx *context.Context) {
	if !oauthDoerAuthorizePreCheck(ctx, "") {
		return
	}

	form := web.GetForm[*forms.DeviceGrantApplicationForm](ctx)
	if ctx.Session.Get(oauthDeviceAuthorizationIDKey) != strconv.FormatInt(form.DeviceAuthorizationID, 10) {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(ctx, form.DeviceAuthorizationID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.ServerError("GetOAuth2DeviceAuthorizationByID", err)
		return
	}
	if err != nil || deviceAuthorization.IsExpired() {
		renderOAuthDeviceAuthorizationError(ctx, "auth.device_code_invalid")
		return
	}
	if deviceAuthorization.Status != auth_model.OAuth2DeviceAuthorizationPending {
		if deviceAuthorization.Status == auth_model.OAuth2DeviceAuthorizationDenied {
			renderOAuthDeviceAuthorizationComplete(ctx, false)
			return
		}
		renderOAuthDeviceAuthorizationComplete(ctx, true)
		return
	}

	if !form.Granted {
		if err := deviceAuthorization.MarkDenied(ctx, ctx.Doer.ID); err != nil {
			handleDeviceAuthorizationWriteError(ctx, "MarkDenied", deviceAuthorization.ID, err)
			return
		}
		renderOAuthDeviceAuthorizationComplete(ctx, false)
		return
	}

	app, err := auth_model.GetOAuth2ApplicationByID(ctx, deviceAuthorization.ApplicationID)
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}

	// the grant is created outside the transaction: on PostgreSQL a failed insert aborts it, so the fallback lookup could not run
	grant, err := app.GetGrantByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetGrantByUserID", err)
		return
	}
	if grant == nil {
		var createErr error
		if grant, createErr = app.CreateGrant(ctx, ctx.Doer.ID, deviceAuthorization.Scope); createErr != nil {
			// a concurrent request may have created the grant, fall back to loading it
			grant, err = app.GetGrantByUserID(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("GetGrantByUserID", err)
				return
			}
			if grant == nil {
				ctx.ServerError("CreateGrant", createErr)
				return
			}
		}
	}
	if grant.Scope != deviceAuthorization.Scope {
		if err := deviceAuthorization.MarkDenied(ctx, ctx.Doer.ID); err != nil {
			handleDeviceAuthorizationWriteError(ctx, "MarkDenied", deviceAuthorization.ID, err)
			return
		}
		renderOAuthDeviceAuthorizationError(ctx, "auth.device_scope_mismatch")
		return
	}

	if err := db.WithTx(ctx, func(txCtx stdctx.Context) error {
		deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(txCtx, form.DeviceAuthorizationID)
		if errors.Is(err, util.ErrNotExist) || (err == nil && deviceAuthorization.IsExpired()) {
			return auth_model.ErrOAuth2DeviceAuthorizationInvalidated
		} else if err != nil {
			return err
		}
		return deviceAuthorization.MarkApproved(txCtx, grant.ID, ctx.Doer.ID)
	}); err != nil {
		handleDeviceAuthorizationWriteError(ctx, "approveDeviceAuthorization", form.DeviceAuthorizationID, err)
		return
	}

	renderOAuthDeviceAuthorizationComplete(ctx, true)
}

func handleDeviceCode(ctx *context.Context, form forms.AccessTokenForm, serverKey, clientKey oauth2_provider.JWTSigningKey) {
	app := loadPublicOAuth2App(ctx, form.ClientID)
	if app == nil {
		return
	}

	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByDeviceCode(ctx, form.DeviceCode)
	if errors.Is(err, util.ErrNotExist) {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code is invalid",
		})
		return
	} else if err != nil {
		handleDeviceAccessTokenServerError(ctx, "GetOAuth2DeviceAuthorizationByDeviceCode", err)
		return
	}
	if deviceAuthorization.ApplicationID != app.ID {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code is invalid",
		})
		return
	}
	if deviceAuthorization.IsExpired() {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeExpiredToken,
			ErrorDescription: "device code expired",
		})
		return
	}

	switch deviceAuthorization.Status {
	case auth_model.OAuth2DeviceAuthorizationPending:
		slowDown, err := deviceAuthorization.RegisterPoll(ctx)
		if err != nil {
			handleDeviceAccessTokenServerError(ctx, "RegisterPoll", err)
			return
		}
		if slowDown {
			handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
				ErrorCode:        oauth2_provider.AccessTokenErrorCodeSlowDown,
				ErrorDescription: "polling too quickly",
			})
			return
		}
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeAuthorizationPending,
			ErrorDescription: "device authorization pending",
		})
		return
	case auth_model.OAuth2DeviceAuthorizationDenied:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeAccessDenied,
			ErrorDescription: "device authorization denied",
		})
		return
	case auth_model.OAuth2DeviceAuthorizationConsumed:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code already used",
		})
		return
	}

	var resp *oauth2_provider.AccessTokenResponse
	err = db.WithTx(ctx, func(txCtx stdctx.Context) error {
		deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(txCtx, deviceAuthorization.ID)
		if errors.Is(err, util.ErrNotExist) || (err == nil && deviceAuthorization.IsExpired()) {
			return auth_model.ErrOAuth2DeviceAuthorizationInvalidated
		} else if err != nil {
			return err
		}
		if err := deviceAuthorization.MarkConsumed(txCtx); err != nil {
			return err
		}

		grant, err := auth_model.GetOAuth2GrantByID(txCtx, deviceAuthorization.GrantID)
		if err != nil {
			return err
		}
		if grant == nil {
			return &oauth2_provider.AccessTokenError{
				ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
				ErrorDescription: "grant does not exist",
			}
		}
		if grant.ApplicationID != app.ID {
			return &oauth2_provider.AccessTokenError{
				ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
				ErrorDescription: "device code belongs to a different client",
			}
		}

		var tokenErr *oauth2_provider.AccessTokenError
		resp, tokenErr = oauth2_provider.NewAccessTokenResponse(txCtx, grant, serverKey, clientKey)
		if tokenErr != nil {
			return tokenErr
		}

		return nil
	})
	if err != nil {
		var accessTokenErr *oauth2_provider.AccessTokenError
		switch {
		case errors.As(err, &accessTokenErr) && accessTokenErr != nil:
			handleAccessTokenError(ctx, *accessTokenErr)
		case errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationInvalidated):
			if err := handleCurrentOAuthDeviceAuthorizationTokenState(ctx, deviceAuthorization.ID); err != nil {
				handleDeviceAccessTokenServerError(ctx, "handleCurrentOAuthDeviceAuthorizationTokenState", err)
			}
		default:
			handleDeviceAccessTokenServerError(ctx, "consumeDeviceAuthorization", err)
		}
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// handleDeviceAccessTokenServerError keeps the token endpoint on JSON, since polling clients cannot parse an HTML error page.
func handleDeviceAccessTokenServerError(ctx *context.Context, name string, err error) {
	log.Error("%s: %v", name, err)
	ctx.JSON(http.StatusInternalServerError, oauth2_provider.AccessTokenError{
		ErrorCode:        oauth2_provider.AccessTokenErrorCodeServerError,
		ErrorDescription: "an internal error occurred",
	})
}

func renderOAuthDeviceAuthorizationError(ctx *context.Context, msgKey string) {
	ctx.Data["Error"] = AuthorizeError{ErrorDescription: ctx.Locale.TrString(msgKey)}
	ctx.HTML(http.StatusBadRequest, tplGrantError)
}

// handleDeviceAuthorizationWriteError renders the authorization's current state when it changed
// under us, and reports anything else as a server error.
func handleDeviceAuthorizationWriteError(ctx *context.Context, name string, deviceAuthorizationID int64, err error) {
	if !errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationInvalidated) {
		ctx.ServerError(name, err)
		return
	}
	if err := renderCurrentOAuthDeviceAuthorizationResult(ctx, deviceAuthorizationID); err != nil {
		ctx.ServerError("renderCurrentOAuthDeviceAuthorizationResult", err)
	}
}

func renderOAuthDeviceAuthorizationEntry(ctx *context.Context, userCode string) {
	ctx.Data["Title"] = ctx.Tr("auth.device_code_entry_title")
	ctx.Data["UserCode"] = userCode
	ctx.HTML(http.StatusOK, tplDeviceAuthorization)
}

func renderOAuthDeviceAuthorizationComplete(ctx *context.Context, granted bool) {
	if granted {
		ctx.Data["Title"] = ctx.Tr("auth.device_authorization_complete_title")
	} else {
		ctx.Data["Title"] = ctx.Tr("auth.device_authorization_cancelled_title")
	}
	ctx.Data["Granted"] = granted
	ctx.HTML(http.StatusOK, tplDeviceAuthorizationComplete)
}

func renderCurrentOAuthDeviceAuthorizationResult(ctx *context.Context, deviceAuthorizationID int64) error {
	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(ctx, deviceAuthorizationID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return err
	}
	if err != nil || deviceAuthorization.IsExpired() {
		renderOAuthDeviceAuthorizationError(ctx, "auth.device_code_invalid")
		return nil
	}

	switch deviceAuthorization.Status {
	case auth_model.OAuth2DeviceAuthorizationDenied:
		renderOAuthDeviceAuthorizationComplete(ctx, false)
	case auth_model.OAuth2DeviceAuthorizationApproved, auth_model.OAuth2DeviceAuthorizationConsumed:
		renderOAuthDeviceAuthorizationComplete(ctx, true)
	default:
		renderOAuthDeviceAuthorizationError(ctx, "auth.device_code_invalid")
	}
	return nil
}

func handleCurrentOAuthDeviceAuthorizationTokenState(ctx *context.Context, deviceAuthorizationID int64) error {
	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(ctx, deviceAuthorizationID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return err
	}
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code is invalid",
		})
		return nil
	}
	if deviceAuthorization.IsExpired() {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeExpiredToken,
			ErrorDescription: "device code expired",
		})
		return nil
	}

	switch deviceAuthorization.Status {
	case auth_model.OAuth2DeviceAuthorizationPending:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeAuthorizationPending,
			ErrorDescription: "device authorization pending",
		})
	case auth_model.OAuth2DeviceAuthorizationDenied:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeAccessDenied,
			ErrorDescription: "device authorization denied",
		})
	case auth_model.OAuth2DeviceAuthorizationConsumed:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code already used",
		})
	default:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code is invalid",
		})
	}
	return nil
}

func setOAuthDeviceAuthorizationData(ctx *context.Context, app *auth_model.OAuth2Application, deviceAuthorization *auth_model.OAuth2DeviceAuthorization) error {
	ctx.Data["Title"] = ctx.Tr("auth.authorize_title", app.Name)
	ctx.Data["Application"] = app
	ctx.Data["DeviceAuthorization"] = deviceAuthorization
	ctx.Data["AdditionalScopes"] = oauth2_provider.GrantAdditionalScopes(deviceAuthorization.Scope) != auth_model.AccessTokenScopeAll
	ctx.Data["Scope"] = deviceAuthorization.Scope
	var user *user_model.User
	if app.UID != 0 {
		var err error
		user, err = user_model.GetUserByID(ctx, app.UID)
		if err != nil {
			return err
		}
	}
	ctx.Data["ApplicationCreatorLinkHTML"] = oauthApplicationCreatorLinkHTML(user)
	return nil
}
