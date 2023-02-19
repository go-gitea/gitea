// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"strconv"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"

	"github.com/golang-jwt/jwt/v4"
)

// NewAccessToken creates a new AccessToken with the Token field containing a signed token string
func NewAccessToken(accessToken *auth.AccessToken) error {
	err := auth.NewAccessToken(accessToken)
	if err != nil {
		return err
	}
	token := &oauth2.Token{
		GrantID: accessToken.ID,
		Type:    oauth2.TypeAccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "token-" + strconv.FormatInt(accessToken.ID, 10),
		},
	}

	accessToken.Token, err = token.SignToken(oauth2.DefaultSigningKey)
	return err
}
