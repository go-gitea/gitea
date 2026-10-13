// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"slices"
	"strings"

	"gitea.dev/models/auth"
	gitea_context "gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// grantCovers says a grant already holds every scope being asked for, so
// asking the user again would show them nothing they have not already agreed
// to.
//
// Equality is the wrong test. A request that names no scope at all is asking
// for nothing in particular -- the parameter is optional in RFC 6749 Section
// 3.3 -- and one that names a subset is asking for less than it already has.
// Comparing strings turns both into a consent screen, so every client that
// omits the parameter would be re-prompted on each sign-in despite holding a
// perfectly good grant.
func grantCovers(grant *auth.OAuth2Grant, scope string) bool {
	for _, want := range strings.Fields(scope) {
		if !grant.ScopeContains(want) {
			return false
		}
	}
	return true
}

// widenGrant adds to a grant any scope being asked for that it does not
// already hold, and writes it only if that changed something.
//
// A union rather than a replacement, because the token's capabilities are
// read from the grant and not from the request: replacing would let a request
// naming fewer scopes quietly strip capability from an authorization the user
// had already given, and one naming none at all would empty the grant
// outright -- taking "openid" with it, so the next sign-in returns no ID
// token.
func widenGrant(ctx context.Context, grant *auth.OAuth2Grant, scope string) error {
	held := strings.Fields(grant.Scope)
	grew := false
	for _, want := range strings.Fields(scope) {
		if !slices.Contains(held, want) {
			held = append(held, want)
			grew = true
		}
	}
	if !grew {
		return nil
	}
	return grant.SetScope(ctx, strings.Join(held, " "))
}

// settleFirstPartyGrant brings the grant of an application flagged
// SkipSecondaryAuthorization up to what is being asked for, without a consent
// screen: creating it on the first authorization, and widening it when a
// scope is asked for that it does not hold.
//
// Such an application is trusted outright. This is for the first-party client
// that *is* the site being signed in to, where asking somebody to authorize
// the application they are already looking at tells them nothing they can act
// on -- there is no third party, and declining leaves them unable to use the
// product they just opened. RFC 6749 Section 10.2 asks for consent to protect
// a user from a client they have not chosen, which is not this case; the
// operator chose it when they set the flag, and only an administrator can.
// Without the widening, a consent screen would appear once per release that
// touches scopes.
//
// Confidential clients without the flag are left alone and keep the old
// behaviour, consent on first use, because confidentiality alone does not say
// first-party.
//
// Returns the grant to carry on with. A false second return means an error
// response has already been written and the caller must stop.
func settleFirstPartyGrant(ctx *gitea_context.Context, app *auth.OAuth2Application, grant *auth.OAuth2Grant, form *forms.AuthorizationForm) (*auth.OAuth2Grant, bool) {
	if !app.SkipSecondaryAuthorization {
		return grant, true
	}
	if grant == nil {
		created, err := app.CreateGrant(ctx, ctx.Doer.ID, form.Scope)
		if err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				State:            form.State,
				ErrorDescription: "cannot create grant for user",
				ErrorCode:        ErrorCodeServerError,
			}, form.RedirectURI)
			return nil, false
		}
		return created, true
	}
	if err := widenGrant(ctx, grant, form.Scope); err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return nil, false
	}
	return grant, true
}

// acceptGrantedScope records a scope the user has just been shown and agreed
// to on the consent screen.
//
// Refusing instead was the old behaviour, and it made adding a scope
// permanently impossible: every user who had authorized the application
// before got a server error, and the only way out was revoking it by hand.
//
// A false return means an error response has already been written.
func acceptGrantedScope(ctx *gitea_context.Context, grant *auth.OAuth2Grant, form *forms.GrantApplicationForm) bool {
	if err := widenGrant(ctx, grant, form.Scope); err != nil {
		handleAuthorizeError(ctx, AuthorizeError{
			State:            form.State,
			ErrorDescription: "cannot update grant scope for user",
			ErrorCode:        ErrorCodeServerError,
		}, form.RedirectURI)
		return false
	}
	return true
}
