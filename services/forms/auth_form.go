// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import "gitea.dev/modules/web/middleware"

// AuthenticationForm form for authentication
type AuthenticationForm struct {
	middleware.FormDefaultValidator
	Type            int    `binding:"In(1,2)"`
	Name            string `binding:"Required;MaxSize(30)"`
	TwoFactorPolicy string
	IsActive        bool
	IsSyncEnabled   bool

	// SMTP
	SMTPAuth       string
	SMTPHost       string
	SMTPPort       int
	AllowedDomains string
	SkipVerify     bool
	HeloHostname   string
	DisableHelo    bool
	ForceSMTPS     bool

	// Oauth2 & OIDC
	Oauth2Provider                string
	Oauth2Key                     string
	Oauth2Secret                  string
	OpenIDConnectAutoDiscoveryURL string
	Oauth2UseCustomURL            bool
	Oauth2TokenURL                string
	Oauth2AuthURL                 string
	Oauth2ProfileURL              string
	Oauth2EmailURL                string
	Oauth2IconURL                 string
	Oauth2Tenant                  string
	Oauth2Scopes                  string
	Oauth2RequiredClaimName       string
	Oauth2RequiredClaimValue      string
	Oauth2GroupClaimName          string
	Oauth2AdminGroup              string
	Oauth2RestrictedGroup         string
	Oauth2GroupTeamMap            string `binding:"ValidGroupTeamMap"`
	Oauth2GroupTeamMapRemoval     bool
	Oauth2SSHPublicKeyClaimName   string
	Oauth2FullNameClaimName       string
	OpenIDConnectExternalIDClaim  string
}
