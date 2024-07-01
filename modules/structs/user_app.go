// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// AccessToken represents an API access token.
// swagger:response AccessToken
type AccessToken struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Token          string   `json:"sha1"`
	TokenLastEight string   `json:"token_last_eight"`
	Scopes         []string `json:"scopes"`
}

// AccessTokenList represents a list of API access token.
// swagger:response AccessTokenList
type AccessTokenList []*AccessToken

// CreateAccessTokenOption options when create access token
type CreateAccessTokenOption struct {
	// required: true
	Name   string   `json:"name" binding:"Required"`
	Scopes []string `json:"scopes"`
}

// CreateOAuth2ApplicationOptions holds options to create an oauth2 application
type CreateOAuth2ApplicationOptions struct {
	Name                       string   `json:"name" binding:"Required"`
	ConfidentialClient         bool     `json:"confidential_client"`
	SkipSecondaryAuthorization bool     `json:"skip_secondary_authorization"`
	RedirectURIs               []string `json:"redirect_uris" binding:"Required"`
}

// OAuth2Application represents an OAuth2 application.
// swagger:response OAuth2Application
type OAuth2Application struct {
	ID                         int64     `json:"id"`
	Name                       string    `json:"name"`
	ClientID                   string    `json:"client_id"`
	ClientSecret               string    `json:"client_secret"`
	ConfidentialClient         bool      `json:"confidential_client"`
	SkipSecondaryAuthorization bool      `json:"skip_secondary_authorization"`
	RedirectURIs               []string  `json:"redirect_uris"`
	Created                    time.Time `json:"created"`
}

// OAuth2ApplicationList represents a list of OAuth2 applications.
// swagger:response OAuth2ApplicationList
type OAuth2ApplicationList []*OAuth2Application
