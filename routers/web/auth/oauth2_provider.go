// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/oauth2_provider"

	"gitea.com/go-chi/binding"
	jwt "github.com/golang-jwt/jwt/v5"
)

const (
	tplGrantAccess templates.TplName = "user/auth/grant"
	tplGrantError  templates.TplName = "user/auth/grant_error"
)

// TODO move error and responses to SDK or models

// AuthorizeErrorCode represents an error code specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2.1
type AuthorizeErrorCode string

const (
	// ErrorCodeInvalidRequest represents the according error in RFC 6749
	ErrorCodeInvalidRequest AuthorizeErrorCode = "invalid_request"
	// ErrorCodeUnauthorizedClient represents the according error in RFC 6749
	ErrorCodeUnauthorizedClient AuthorizeErrorCode = "unauthorized_client"
	// ErrorCodeAccessDenied represents the according error in RFC 6749
	ErrorCodeAccessDenied AuthorizeErrorCode = "access_denied"
	// ErrorCodeUnsupportedResponseType represents the according error in RFC 6749
	ErrorCodeUnsupportedResponseType AuthorizeErrorCode = "unsupported_response_type"
	// ErrorCodeInvalidScope represents the according error in RFC 6749
	ErrorCodeInvalidScope AuthorizeErrorCode = "invalid_scope"
	// ErrorCodeServerError represents the according error in RFC 6749
	ErrorCodeServerError AuthorizeErrorCode = "server_error"
	// ErrorCodeTemporaryUnavailable represents the according error in RFC 6749
	ErrorCodeTemporaryUnavailable AuthorizeErrorCode = "temporarily_unavailable"
)

// AuthorizeError represents an error type specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2.1
type AuthorizeError struct {
	ErrorCode        AuthorizeErrorCode `json:"error" form:"error"`
	ErrorDescription string
	State            string
}

// Error returns the error message
func (err AuthorizeError) Error() string {
	return fmt.Sprintf("%s: %s", err.ErrorCode, err.ErrorDescription)
}

// errCallback represents a oauth2 callback error
type errCallback struct {
	Code        string
	Description string
}

func (err errCallback) Error() string {
	return err.Description
}

type userInfoResponse struct {
	Sub               string   `json:"sub"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Picture           string   `json:"picture"`
	Groups            []string `json:"groups"`
}

// InfoOAuth manages request for userinfo endpoint
func InfoOAuth(ctx *context.Context) {
	if ctx.Doer == nil || ctx.Data["AuthedMethod"] != (&auth_service.OAuth2{}).Name() {
		ctx.Resp.Header().Set("WWW-Authenticate", `Bearer realm="Gitea OAuth2"`)
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	response := &userInfoResponse{
		Sub:               fmt.Sprint(ctx.Doer.ID),
		Name:              ctx.Doer.DisplayName(),
		PreferredUsername: ctx.Doer.Name,
		Email:             ctx.Doer.Email,
		Picture:           ctx.Doer.AvatarLink(ctx),
	}

	var accessTokenScope auth.AccessTokenScope
	if auHead := ctx.Req.Header.Get("Authorization"); auHead != "" {
		auths := strings.Fields(auHead)
		if len(auths) == 2 && (auths[0] == "token" || strings.ToLower(auths[0]) == "bearer") {
			accessTokenScope, _ = auth_service.GetOAuthAccessTokenScopeAndUserID(ctx, auths[1])
		}
	}

	// since version 1.22 does not verify if groups should be public-only,
	// onlyPublicGroups will be set only if 'public-only' is included in a valid scope
	onlyPublicGroups, _ := accessTokenScope.PublicOnly()
	groups, err := oauth2_provider.GetOAuthGroupsForUser(ctx, ctx.Doer, onlyPublicGroups)
	if err != nil {
		ctx.ServerError("Oauth groups for user", err)
		return
	}
	response.Groups = groups

	ctx.JSON(http.StatusOK, response)
}

func parseBasicAuth(ctx *context.Context) (username, password string, err error) {
	authHeader := ctx.Req.Header.Get("Authorization")
	if authType, authData, ok := strings.Cut(authHeader, " "); ok && strings.EqualFold(authType, "Basic") {
		return base.BasicAuthDecode(authData)
	}
	return "", "", errors.New("invalid basic authentication")
}

// IntrospectOAuth introspects an oauth token
func IntrospectOAuth(ctx *context.Context) {
	clientIDValid := false
	if clientID, clientSecret, err := parseBasicAuth(ctx); err == nil {
		app, err := auth.GetOAuth2ApplicationByClientID(ctx, clientID)
		if err != nil && !auth.IsErrOauthClientIDInvalid(err) {
			// this is likely a database error; log it and respond without details
			log.Error("Error retrieving client_id: %v", err)
			ctx.Error(http.StatusInternalServerError)
			return
		}
		clientIDValid = err == nil && app.ValidateClientSecret([]byte(clientSecret))
	}
	if !clientIDValid {
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea OAuth2"`)
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	var response struct {
		Active   bool   `json:"active"`
		Scope    string `json:"scope,omitempty"`
		Username string `json:"username,omitempty"`
		jwt.RegisteredClaims
	}

	form := web.GetForm(ctx).(*forms.IntrospectTokenForm)
	token, err := oauth2_provider.ParseToken(form.Token, oauth2_provider.DefaultSigningKey)
	if err == nil {
		grant, err := auth.GetOAuth2GrantByID(ctx, token.GrantID)
		if err == nil && grant != nil {
			app, err := auth.GetOAuth2ApplicationByID(ctx, grant.ApplicationID)
			if err == nil && app != nil {
				response.Active = true
				response.Scope = grant.Scope
				response.Issuer = setting.AppURL
				response.Audience = []string{app.ClientID}
				response.Subject = fmt.Sprint(grant.UserID)
			}
			if user, err := user_model.GetUserByID(ctx, grant.UserID); err == nil {
				response.Username = user.Name
			}
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// AuthorizeOAuth manages authorize requests
func AuthorizeOAuth(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AuthorizationForm)
	errs := binding.Errors{}
	errs = form.Validate(ctx.Req, errs)
	if len(errs) > 0 {
		errstring := ""
		for _, e := range errs {
			errstring += e.Error() + "\n"
		}
		ctx.ServerError("AuthorizeOAuth: Validate: ", fmt.Errorf("errors occurred during validation: %s", errstring))
		return
	}

	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		if auth.IsErrOauthClientIDInvalid(err) {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeUnauthorizedClient,
				ErrorDescription: "Client ID not registered",
				State:            form.State,
			}, "")
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}

	var user *user_model.User
	if app.UID != 0 {
		user, err = user_model.GetUserByID(ctx, app.UID)
		if err != nil {
			ctx.ServerError("GetUserByID", err)
			return
		}
	}

	if !app.ContainsRedirectURI(form.RedirectURI) {
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeInvalidRequest,
			ErrorDescription: "Unregistered Redirect URI",
			State:            form.State,
		}, "")
		return
	}

	if form.ResponseType != "code" {
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeUnsupportedResponseType,
			ErrorDescription: "Only code response type is supported.",
			State:            form.State,
		}, form.RedirectURI)
		return
	}

	// pkce support
	switch form.CodeChallengeMethod {
	case "S256":
	case "plain":
		if err := ctx.Session.Set("CodeChallengeMethod", form.CodeChallengeMethod); err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeServerError,
				ErrorDescription: "cannot set code challenge method",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
		if err := ctx.Session.Set("CodeChallengeMethod", form.CodeChallenge); err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeServerError,
				ErrorDescription: "cannot set code challenge",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
		// Here we're just going to try to release the session early
		if err := ctx.Session.Release(); err != nil {
			// we'll tolerate errors here as they *should* get saved elsewhere
			log.Error("Unable to save changes to the session: %v", err)
		}
	case "":
		// "Authorization servers SHOULD reject authorization requests from native apps that don't use PKCE by returning an error message"
		// https://datatracker.ietf.org/doc/html/rfc8252#section-8.1
		if !app.ConfidentialClient {
			// "the authorization endpoint MUST return the authorization error response with the "error" value set to "invalid_request""
			// https://datatracker.ietf.org/doc/html/rfc7636#section-4.4.1
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeInvalidRequest,
				ErrorDescription: "PKCE is required for public clients",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
	default:
		// "If the server supporting PKCE does not support the requested transformation, the authorization endpoint MUST return the authorization error response with "error" value set to "invalid_request"."
		// https://www.rfc-editor.org/rfc/rfc7636#section-4.4.1
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeInvalidRequest,
			ErrorDescription: "unsupported code challenge method",
			State:            form.State,
		}, form.RedirectURI)
		return
	}

	grant, err := app.GetGrantByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}

	// Redirect if user already granted access and the application is confidential or trusted otherwise
	// I.e. always require authorization for untrusted public clients as recommended by RFC 6749 Section 10.2
	if (app.ConfidentialClient || app.SkipSecondaryAuthorization) && grant != nil {
		code, err := grant.GenerateNewAuthorizationCode(ctx, form.RedirectURI, form.CodeChallenge, form.CodeChallengeMethod)
		if err != nil {
			handleServerError(ctx, form.State, form.RedirectURI)
			return
		}
		redirect, err := code.GenerateRedirectURI(form.State)
		if err != nil {
			handleServerError(ctx, form.State, form.RedirectURI)
			return
		}
		// Update nonce to reflect the new session
		if len(form.Nonce) > 0 {
			err := grant.SetNonce(ctx, form.Nonce)
			if err != nil {
				log.Error("Unable to update nonce: %v", err)
			}
		}
		ctx.Redirect(redirect.String())
		return
	}

	// check if additional scopes
	ctx.Data["AdditionalScopes"] = oauth2_provider.GrantAdditionalScopes(form.Scope) != auth.AccessTokenScopeAll

	// show authorize page to grant access
	ctx.Data["Application"] = app
	ctx.Data["RedirectURI"] = form.RedirectURI
	ctx.Data["State"] = form.State
	ctx.Data["Scope"] = form.Scope
	ctx.Data["Nonce"] = form.Nonce
	if user != nil {
		ctx.Data["ApplicationCreatorLinkHTML"] = template.HTML(fmt.Sprintf(`<a href="%s">@%s</a>`, html.EscapeString(user.HomeLink()), html.EscapeString(user.Name)))
	} else {
		ctx.Data["ApplicationCreatorLinkHTML"] = template.HTML(fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(setting.AppSubURL+"/"), html.EscapeString(setting.AppName)))
	}
	ctx.Data["ApplicationRedirectDomainHTML"] = template.HTML("<strong>" + html.EscapeString(form.RedirectURI) + "</strong>")
	// TODO document SESSION <=> FORM
	err = ctx.Session.Set("client_id", app.ClientID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		log.Error(err.Error())
		return
	}
	err = ctx.Session.Set("redirect_uri", form.RedirectURI)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		log.Error(err.Error())
		return
	}
	err = ctx.Session.Set("state", form.State)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		log.Error(err.Error())
		return
	}
	// Here we're just going to try to release the session early
	if err := ctx.Session.Release(); err != nil {
		// we'll tolerate errors here as they *should* get saved elsewhere
		log.Error("Unable to save changes to the session: %v", err)
	}
	ctx.HTML(http.StatusOK, tplGrantAccess)
}

// GrantApplicationOAuth manages the post request submitted when a user grants access to an application
func GrantApplicationOAuth(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.GrantApplicationForm)
	if ctx.Session.Get("client_id") != form.ClientID || ctx.Session.Get("state") != form.State ||
		ctx.Session.Get("redirect_uri") != form.RedirectURI {
		ctx.Error(http.StatusBadRequest)
		return
	}

	if !form.Granted {
		handleAuthorizeError(ctx, AuthorizeError{
			State:            form.State,
			ErrorDescription: "the request is denied",
			ErrorCode:        ErrorCodeAccessDenied,
		}, form.RedirectURI)
		return
	}

	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}
	grant, err := app.GetGrantByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}
	if grant == nil {
		grant, err = app.CreateGrant(ctx, ctx.Doer.ID, form.Scope)
		if err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				State:            form.State,
				ErrorDescription: "cannot create grant for user",
				ErrorCode:        ErrorCodeServerError,
			}, form.RedirectURI)
			return
		}
	} else if grant.Scope != form.Scope {
		handleAuthorizeError(ctx, AuthorizeError{
			State:            form.State,
			ErrorDescription: "a grant exists with different scope",
			ErrorCode:        ErrorCodeServerError,
		}, form.RedirectURI)
		return
	}

	if len(form.Nonce) > 0 {
		err := grant.SetNonce(ctx, form.Nonce)
		if err != nil {
			log.Error("Unable to update nonce: %v", err)
		}
	}

	var codeChallenge, codeChallengeMethod string
	codeChallenge, _ = ctx.Session.Get("CodeChallenge").(string)
	codeChallengeMethod, _ = ctx.Session.Get("CodeChallengeMethod").(string)

	code, err := grant.GenerateNewAuthorizationCode(ctx, form.RedirectURI, codeChallenge, codeChallengeMethod)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}
	redirect, err := code.GenerateRedirectURI(form.State)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}
	ctx.Redirect(redirect.String(), http.StatusSeeOther)
}

// OIDCWellKnown generates JSON so OIDC clients know Gitea's capabilities
func OIDCWellKnown(ctx *context.Context) {
	ctx.Data["SigningKey"] = oauth2_provider.DefaultSigningKey
	ctx.JSONTemplate("user/auth/oidc_wellknown")
}

// OIDCKeys generates the JSON Web Key Set
func OIDCKeys(ctx *context.Context) {
	jwk, err := oauth2_provider.DefaultSigningKey.ToJWK()
	if err != nil {
		log.Error("Error converting signing key to JWK: %v", err)
		ctx.Error(http.StatusInternalServerError)
		return
	}

	jwk["use"] = "sig"

	jwks := map[string][]map[string]string{
		"keys": {
			jwk,
		},
	}

	ctx.Resp.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(jwks); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
}

// AccessTokenOAuth manages all access token requests by the client
func AccessTokenOAuth(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.AccessTokenForm)
	// if there is no ClientID or ClientSecret in the request body, fill these fields by the Authorization header and ensure the provided field matches the Authorization header
	if form.ClientID == "" || form.ClientSecret == "" {
		authHeader := ctx.Req.Header.Get("Authorization")
		if authType, authData, ok := strings.Cut(authHeader, " "); ok && strings.EqualFold(authType, "Basic") {
			clientID, clientSecret, err := base.BasicAuthDecode(authData)
			if err != nil {
				handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
					ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot parse basic auth header",
				})
				return
			}
			// validate that any fields present in the form match the Basic auth header
			if form.ClientID != "" && form.ClientID != clientID {
				handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
					ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "client_id in request body inconsistent with Authorization header",
				})
				return
			}
			form.ClientID = clientID
			if form.ClientSecret != "" && form.ClientSecret != clientSecret {
				handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
					ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "client_secret in request body inconsistent with Authorization header",
				})
				return
			}
			form.ClientSecret = clientSecret
		}
	}

	serverKey := oauth2_provider.DefaultSigningKey
	clientKey := serverKey
	if serverKey.IsSymmetric() {
		var err error
		clientKey, err = oauth2_provider.CreateJWTSigningKey(serverKey.SigningMethod().Alg(), []byte(form.ClientSecret))
		if err != nil {
			handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
				ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "Error creating signing key",
			})
			return
		}
	}

	switch form.GrantType {
	case "refresh_token":
		handleRefreshToken(ctx, form, serverKey, clientKey)
	case "authorization_code":
		handleAuthorizationCode(ctx, form, serverKey, clientKey)
	default:
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnsupportedGrantType,
			ErrorDescription: "Only refresh_token or authorization_code grant type is supported",
		})
	}
}

func handleRefreshToken(ctx *context.Context, form forms.AccessTokenForm, serverKey, clientKey oauth2_provider.JWTSigningKey) {
	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: %q", form.ClientID),
		})
		return
	}
	// "The authorization server MUST ... require client authentication for confidential clients"
	// https://datatracker.ietf.org/doc/html/rfc6749#section-6
	if app.ConfidentialClient && !app.ValidateClientSecret([]byte(form.ClientSecret)) {
		errorDescription := "invalid client secret"
		if form.ClientSecret == "" {
			errorDescription = "invalid empty client secret"
		}
		// "invalid_client ... Client authentication failed"
		// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidClient,
			ErrorDescription: errorDescription,
		})
		return
	}

	token, err := oauth2_provider.ParseToken(form.RefreshToken, serverKey)
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "unable to parse refresh token",
		})
		return
	}
	// get grant before increasing counter
	grant, err := auth.GetOAuth2GrantByID(ctx, token.GrantID)
	if err != nil || grant == nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "grant does not exist",
		})
		return
	}

	// check if token got already used
	if setting.OAuth2.InvalidateRefreshTokens && (grant.Counter != token.Counter || token.Counter == 0) {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "token was already used",
		})
		log.Warn("A client tried to use a refresh token for grant_id = %d was used twice!", grant.ID)
		return
	}
	accessToken, tokenErr := oauth2_provider.NewAccessTokenResponse(ctx, grant, serverKey, clientKey)
	if tokenErr != nil {
		handleAccessTokenError(ctx, *tokenErr)
		return
	}
	ctx.JSON(http.StatusOK, accessToken)
}

func handleAuthorizationCode(ctx *context.Context, form forms.AccessTokenForm, serverKey, clientKey oauth2_provider.JWTSigningKey) {
	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: '%s'", form.ClientID),
		})
		return
	}
	if app.ConfidentialClient && !app.ValidateClientSecret([]byte(form.ClientSecret)) {
		errorDescription := "invalid client secret"
		if form.ClientSecret == "" {
			errorDescription = "invalid empty client secret"
		}
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: errorDescription,
		})
		return
	}
	if form.RedirectURI != "" && !app.ContainsRedirectURI(form.RedirectURI) {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "unexpected redirect URI",
		})
		return
	}
	authorizationCode, err := auth.GetOAuth2AuthorizationByCode(ctx, form.Code)
	if err != nil || authorizationCode == nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	// check if code verifier authorizes the client, PKCE support
	if !authorizationCode.ValidateCodeChallenge(form.CodeVerifier) {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "failed PKCE code challenge",
		})
		return
	}
	// check if granted for this application
	if authorizationCode.Grant.ApplicationID != app.ID {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "invalid grant",
		})
		return
	}
	// remove token from database to deny duplicate usage
	if err := authorizationCode.Invalidate(ctx); err != nil {
		handleAccessTokenError(ctx, oauth2_provider.AccessTokenError{
			ErrorCode:        oauth2_provider.AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot proceed your request",
		})
	}
	resp, tokenErr := oauth2_provider.NewAccessTokenResponse(ctx, authorizationCode.Grant, serverKey, clientKey)
	if tokenErr != nil {
		handleAccessTokenError(ctx, *tokenErr)
		return
	}
	// send successful response
	ctx.JSON(http.StatusOK, resp)
}

func handleAccessTokenError(ctx *context.Context, acErr oauth2_provider.AccessTokenError) {
	ctx.JSON(http.StatusBadRequest, acErr)
}

func handleServerError(ctx *context.Context, state, redirectURI string) {
	handleAuthorizeError(ctx, AuthorizeError{
		ErrorCode:        ErrorCodeServerError,
		ErrorDescription: "A server error occurred",
		State:            state,
	}, redirectURI)
}

func handleAuthorizeError(ctx *context.Context, authErr AuthorizeError, redirectURI string) {
	if redirectURI == "" {
		log.Warn("Authorization failed: %v", authErr.ErrorDescription)
		ctx.Data["Error"] = authErr
		ctx.HTML(http.StatusBadRequest, tplGrantError)
		return
	}
	redirect, err := url.Parse(redirectURI)
	if err != nil {
		ctx.ServerError("url.Parse", err)
		return
	}
	q := redirect.Query()
	q.Set("error", string(authErr.ErrorCode))
	q.Set("error_description", authErr.ErrorDescription)
	q.Set("state", authErr.State)
	redirect.RawQuery = q.Encode()
	ctx.Redirect(redirect.String(), http.StatusSeeOther)
}
