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

// DeviceAuthorizationOAuth issues a device code to a public OAuth client.
func DeviceAuthorizationOAuth(ctx *context.Context) {
	form := web.GetForm[*forms.DeviceAuthorizationForm](ctx)
	app, err := auth_model.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: %q", form.ClientID),
		})
		return
	}
	if app.ConfidentialClient {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "device authorization is only supported for public clients",
		})
		return
	}

	deviceAuthorization, deviceCode, err := auth_model.CreateOAuth2DeviceAuthorization(ctx, app, form.Scope)
	if err != nil {
		if errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationLimitReached) {
			handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
				ErrorCode:        oauth2_provider.AccessTokenErrorCodeSlowDown,
				ErrorDescription: "too many pending device authorizations for this client",
			})
			return
		}
		log.Error("CreateOAuth2DeviceAuthorization: %v", err)
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeServerError,
			ErrorDescription: "cannot create device authorization",
		})
		return
	}

	verificationURI := strings.TrimSuffix(setting.AppURL, "/") + "/login/oauth/device"
	formattedUserCode := deviceAuthorization.FormattedUserCode()
	expiresIn := int64(deviceAuthorization.ExpiresAtUnix - timeutil.TimeStampNow())
	expiresIn = max(expiresIn, 0)

	ctx.JSON(http.StatusOK, oauth2_provider.DeviceAuthorizationResponse{
		DeviceCode:              deviceCode,
		UserCode:                formattedUserCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + url.QueryEscape(formattedUserCode),
		ExpiresIn:               expiresIn,
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

// DeviceVerifyOAuth processes the device verification form (POST).
func DeviceVerifyOAuth(ctx *context.Context) {
	form := web.GetForm[*forms.DeviceVerificationForm](ctx)
	userCode := auth_model.NormalizeOAuth2DeviceUserCode(form.UserCode)
	if userCode == "" {
		renderOAuthDeviceAuthorizationEntry(ctx, form.UserCode)
		return
	}

	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByUserCode(ctx, userCode)
	if err != nil {
		ctx.ServerError("GetOAuth2DeviceAuthorizationByUserCode", err)
		return
	}
	if deviceAuthorization == nil || deviceAuthorization.IsExpired() {
		ctx.Flash.Error(ctx.Tr("auth.device_code_invalid"), true)
		renderOAuthDeviceAuthorizationEntry(ctx, form.UserCode)
		return
	}

	switch deviceAuthorization.Status {
	case auth_model.OAuth2DeviceAuthorizationDenied:
		ctx.Flash.Error(ctx.Tr("auth.device_code_denied"), true)
		renderOAuthDeviceAuthorizationEntry(ctx, form.UserCode)
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
	// consent cannot succeed against a grant with a different scope, so say so before asking for it
	if grant != nil && grant.Scope != deviceAuthorization.Scope {
		renderOAuthDeviceAuthorizationScopeMismatch(ctx)
		return
	}

	if grant != nil && app.SkipSecondaryAuthorization {
		if err := deviceAuthorization.MarkApproved(ctx, grant.ID, ctx.Doer.ID); err != nil {
			if errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationInvalidated) {
				if err := renderCurrentOAuthDeviceAuthorizationResult(ctx, deviceAuthorization.ID); err != nil {
					ctx.ServerError("renderCurrentOAuthDeviceAuthorizationResult", err)
				}
				return
			}
			ctx.ServerError("MarkApproved", err)
			return
		}
		renderOAuthDeviceAuthorizationComplete(ctx, true)
		return
	}

	if err := setOAuthDeviceAuthorizationData(ctx, app, deviceAuthorization); err != nil {
		ctx.ServerError("setOAuthDeviceAuthorizationData", err)
		return
	}
	if err := ctx.Session.Set(oauthDeviceAuthorizationIDKey, strconv.FormatInt(deviceAuthorization.ID, 10)); err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Unable to save changes to the session: %v", err)
	}
	ctx.HTML(http.StatusOK, tplDeviceAuthorization)
}

// DeviceGrantApplicationOAuth stores the user's device-flow consent decision.
func DeviceGrantApplicationOAuth(ctx *context.Context) {
	form := web.GetForm[*forms.DeviceGrantApplicationForm](ctx)
	if ctx.Session.Get(oauthDeviceAuthorizationIDKey) != strconv.FormatInt(form.DeviceAuthorizationID, 10) {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(ctx, form.DeviceAuthorizationID)
	if err != nil {
		ctx.ServerError("GetOAuth2DeviceAuthorizationByID", err)
		return
	}
	if deviceAuthorization == nil || deviceAuthorization.IsExpired() {
		ctx.Data["Error"] = AuthorizeError{ErrorDescription: ctx.Locale.TrString("auth.device_code_invalid")}
		ctx.HTML(http.StatusBadRequest, tplGrantError)
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
			if errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationInvalidated) {
				if err := renderCurrentOAuthDeviceAuthorizationResult(ctx, deviceAuthorization.ID); err != nil {
					ctx.ServerError("renderCurrentOAuthDeviceAuthorizationResult", err)
				}
				return
			}
			ctx.ServerError("MarkDenied", err)
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
		renderOAuthDeviceAuthorizationScopeMismatch(ctx)
		return
	}

	if err := db.WithTx(ctx, func(txCtx stdctx.Context) error {
		deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(txCtx, form.DeviceAuthorizationID)
		if err != nil {
			return err
		}
		if deviceAuthorization == nil || deviceAuthorization.IsExpired() {
			return auth_model.ErrOAuth2DeviceAuthorizationInvalidated
		}
		return deviceAuthorization.MarkApproved(txCtx, grant.ID, ctx.Doer.ID)
	}); err != nil {
		switch {
		case errors.Is(err, auth_model.ErrOAuth2DeviceAuthorizationInvalidated):
			if err := renderCurrentOAuthDeviceAuthorizationResult(ctx, form.DeviceAuthorizationID); err != nil {
				ctx.ServerError("renderCurrentOAuthDeviceAuthorizationResult", err)
			}
		default:
			ctx.ServerError("approveDeviceAuthorization", err)
		}
		return
	}

	renderOAuthDeviceAuthorizationComplete(ctx, true)
}

func handleDeviceCode(ctx *context.Context, form forms.AccessTokenForm, serverKey, clientKey oauth2_provider.JWTSigningKey) {
	app, err := auth_model.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: %q", form.ClientID),
		})
		return
	}
	if app.ConfidentialClient {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "device authorization is only supported for public clients",
		})
		return
	}

	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByDeviceCode(ctx, form.DeviceCode)
	if err != nil {
		handleDeviceAccessTokenServerError(ctx, "GetOAuth2DeviceAuthorizationByDeviceCode", err)
		return
	}
	if deviceAuthorization == nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "device code is invalid",
		})
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
		if err != nil {
			return err
		}
		if deviceAuthorization == nil || deviceAuthorization.IsExpired() {
			return auth_model.ErrOAuth2DeviceAuthorizationInvalidated
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
	handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
		ErrorCode:        oauth2_provider.AccessTokenErrorCodeServerError,
		ErrorDescription: "an internal error occurred",
	})
}

func renderOAuthDeviceAuthorizationScopeMismatch(ctx *context.Context) {
	ctx.Data["Error"] = AuthorizeError{ErrorDescription: ctx.Locale.TrString("auth.device_scope_mismatch")}
	ctx.HTML(http.StatusBadRequest, tplGrantError)
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
	if err != nil {
		return err
	}
	if deviceAuthorization == nil || deviceAuthorization.IsExpired() {
		ctx.Data["Error"] = AuthorizeError{ErrorDescription: ctx.Locale.TrString("auth.device_code_invalid")}
		ctx.HTML(http.StatusBadRequest, tplGrantError)
		return nil
	}

	switch deviceAuthorization.Status {
	case auth_model.OAuth2DeviceAuthorizationDenied:
		renderOAuthDeviceAuthorizationComplete(ctx, false)
	case auth_model.OAuth2DeviceAuthorizationApproved, auth_model.OAuth2DeviceAuthorizationConsumed:
		renderOAuthDeviceAuthorizationComplete(ctx, true)
	default:
		ctx.Data["Error"] = AuthorizeError{ErrorDescription: ctx.Locale.TrString("auth.device_code_invalid")}
		ctx.HTML(http.StatusBadRequest, tplGrantError)
	}
	return nil
}

func handleCurrentOAuthDeviceAuthorizationTokenState(ctx *context.Context, deviceAuthorizationID int64) error {
	deviceAuthorization, err := auth_model.GetOAuth2DeviceAuthorizationByID(ctx, deviceAuthorizationID)
	if err != nil {
		return err
	}
	if deviceAuthorization == nil {
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
