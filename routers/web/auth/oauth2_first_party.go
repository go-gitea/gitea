// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"gitea.dev/models/auth"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// settleFirstPartyGrant brings the grant of an application flagged
// SkipSecondaryAuthorization up to what is being asked for, without a consent
// screen: creating it on the first authorization, and widening it when the
// scope has changed.
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
func settleFirstPartyGrant(ctx *context.Context, app *auth.OAuth2Application, grant *auth.OAuth2Grant, form *forms.AuthorizationForm) (*auth.OAuth2Grant, bool) {
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
	if grant.Scope != form.Scope {
		if err := grant.SetScope(ctx, form.Scope); err != nil {
			handleServerError(ctx, form.State, form.RedirectURI)
			return nil, false
		}
	}
	return grant, true
}
