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
	// The unique identifier of the access token
	ID int64 `json:"id"`
	// The name of the access token
	Name string `json:"name"`
	// The SHA1 hash of the access token
	Token string `json:"sha1"`
	// The last eight characters of the token
	TokenLastEight string `json:"token_last_eight"`
	// The scopes granted to this access token
	Scopes []string `json:"scopes"`
	// The timestamp when the token was created
	Created time.Time `json:"created_at"`
	// The timestamp when the token was last used
	Updated time.Time `json:"last_used_at"`
}

// AccessTokenList represents a list of API access token.
// swagger:response AccessTokenList
type AccessTokenList []*AccessToken

// CreateAccessTokenOption options when create access token
// swagger:model CreateAccessTokenOption
type CreateAccessTokenOption struct {
	// required: true
	Name string `json:"name" binding:"Required"`
	// example: ["all", "read:activitypub","read:issue", "write:misc", "read:notification", "read:organization", "read:package", "read:repository", "read:user"]
	Scopes []string `json:"scopes"`
}

// CreateOAuth2ApplicationOptions holds options to create an oauth2 application
type CreateOAuth2ApplicationOptions struct {
	// The name of the OAuth2 application
	Name string `json:"name" binding:"Required"`
	// Whether the client is confidential
	ConfidentialClient bool `json:"confidential_client"`
	// Whether to skip secondary authorization
	SkipSecondaryAuthorization bool `json:"skip_secondary_authorization"`
	// The list of allowed redirect URIs
	RedirectURIs []string `json:"redirect_uris" binding:"Required"`
}

// OAuth2Application represents an OAuth2 application.
// swagger:response OAuth2Application
type OAuth2Application struct {
	// The unique identifier of the OAuth2 application
	ID int64 `json:"id"`
	// The name of the OAuth2 application
	Name string `json:"name"`
	// The client ID of the OAuth2 application
	ClientID string `json:"client_id"`
	// The client secret of the OAuth2 application
	ClientSecret string `json:"client_secret"`
	// Whether the client is confidential
	ConfidentialClient bool `json:"confidential_client"`
	// Whether to skip secondary authorization
	SkipSecondaryAuthorization bool `json:"skip_secondary_authorization"`
	// The list of allowed redirect URIs
	RedirectURIs []string `json:"redirect_uris"`
	// The timestamp when the application was created
	Created time.Time `json:"created"`
}

// OAuth2ApplicationList represents a list of OAuth2 applications.
// swagger:response OAuth2ApplicationList
type OAuth2ApplicationList []*OAuth2Application
