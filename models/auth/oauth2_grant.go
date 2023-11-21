// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

//////////////////////////////////////////////////////

// OAuth2AuthorizationCode is a code to obtain an access token in combination with the client secret once. It has a limited lifetime.
type OAuth2AuthorizationCode struct {
	ID                  int64        `xorm:"pk autoincr"`
	Grant               *OAuth2Grant `xorm:"-"`
	GrantID             int64
	Code                string `xorm:"INDEX unique"`
	CodeChallenge       string
	CodeChallengeMethod string
	RedirectURI         string
	ValidUntil          timeutil.TimeStamp `xorm:"index"`
}

// TableName sets the table name to `oauth2_authorization_code`
func (code *OAuth2AuthorizationCode) TableName() string {
	return "oauth2_authorization_code"
}

// GenerateRedirectURI generates a redirect URI for a successful authorization request. State will be used if not empty.
func (code *OAuth2AuthorizationCode) GenerateRedirectURI(state string) (*url.URL, error) {
	redirect, err := url.Parse(code.RedirectURI)
	if err != nil {
		return nil, err
	}
	q := redirect.Query()
	if state != "" {
		q.Set("state", state)
	}
	q.Set("code", code.Code)
	redirect.RawQuery = q.Encode()
	return redirect, err
}

// Invalidate deletes the auth code from the database to invalidate this code
func (code *OAuth2AuthorizationCode) Invalidate(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(code.ID).NoAutoCondition().Delete(code)
	return err
}

// ValidateCodeChallenge validates the given verifier against the saved code challenge. This is part of the PKCE implementation.
func (code *OAuth2AuthorizationCode) ValidateCodeChallenge(verifier string) bool {
	switch code.CodeChallengeMethod {
	case "S256":
		// base64url(SHA256(verifier)) see https://tools.ietf.org/html/rfc7636#section-4.6
		h := sha256.Sum256([]byte(verifier))
		hashedVerifier := base64.RawURLEncoding.EncodeToString(h[:])
		return hashedVerifier == code.CodeChallenge
	case "plain":
		return verifier == code.CodeChallenge
	case "":
		return true
	default:
		// unsupported method -> return false
		return false
	}
}

// GetOAuth2AuthorizationByCode returns an authorization by its code
func GetOAuth2AuthorizationByCode(ctx context.Context, code string) (auth *OAuth2AuthorizationCode, err error) {
	auth = new(OAuth2AuthorizationCode)
	if has, err := db.GetEngine(ctx).Where("code = ?", code).Get(auth); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	auth.Grant = new(OAuth2Grant)
	if has, err := db.GetEngine(ctx).ID(auth.GrantID).Get(auth.Grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return auth, nil
}

//////////////////////////////////////////////////////

// OAuth2Grant represents the permission of an user for a specific application to access resources
type OAuth2Grant struct {
	ID            int64              `xorm:"pk autoincr"`
	UserID        int64              `xorm:"INDEX unique(user_application)"`
	Application   *OAuth2Application `xorm:"-"`
	ApplicationID int64              `xorm:"INDEX unique(user_application)"`
	Counter       int64              `xorm:"NOT NULL DEFAULT 1"`
	Scope         string             `xorm:"TEXT"`
	Nonce         string             `xorm:"TEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name to `oauth2_grant`
func (grant *OAuth2Grant) TableName() string {
	return "oauth2_grant"
}

// GenerateNewAuthorizationCode generates a new authorization code for a grant and saves it to the database
func (grant *OAuth2Grant) GenerateNewAuthorizationCode(ctx context.Context, redirectURI, codeChallenge, codeChallengeMethod string) (code *OAuth2AuthorizationCode, err error) {
	rBytes, err := util.CryptoRandomBytes(32)
	if err != nil {
		return &OAuth2AuthorizationCode{}, err
	}
	// Add a prefix to the base32, this is in order to make it easier
	// for code scanners to grab sensitive tokens.
	codeSecret := "gta_" + base32Lower.EncodeToString(rBytes)

	code = &OAuth2AuthorizationCode{
		Grant:               grant,
		GrantID:             grant.ID,
		RedirectURI:         redirectURI,
		Code:                codeSecret,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	}
	if err := db.Insert(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}

// IncreaseCounter increases the counter and updates the grant
func (grant *OAuth2Grant) IncreaseCounter(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(grant.ID).Incr("counter").Update(new(OAuth2Grant))
	if err != nil {
		return err
	}
	updatedGrant, err := GetOAuth2GrantByID(ctx, grant.ID)
	if err != nil {
		return err
	}
	grant.Counter = updatedGrant.Counter
	return nil
}

// ScopeContains returns true if the grant scope contains the specified scope
func (grant *OAuth2Grant) ScopeContains(scope string) bool {
	for _, currentScope := range strings.Split(grant.Scope, " ") {
		if scope == currentScope {
			return true
		}
	}
	return false
}

// SetNonce updates the current nonce value of a grant
func (grant *OAuth2Grant) SetNonce(ctx context.Context, nonce string) error {
	grant.Nonce = nonce
	_, err := db.GetEngine(ctx).ID(grant.ID).Cols("nonce").Update(grant)
	if err != nil {
		return err
	}
	return nil
}

// GetOAuth2GrantByID returns the grant with the given ID
func GetOAuth2GrantByID(ctx context.Context, id int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := db.GetEngine(ctx).ID(id).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, err
}

// GetOAuth2GrantsByUserID lists all grants of a certain user
func GetOAuth2GrantsByUserID(ctx context.Context, uid int64) ([]*OAuth2Grant, error) {
	type joinedOAuth2Grant struct {
		Grant       *OAuth2Grant       `xorm:"extends"`
		Application *OAuth2Application `xorm:"extends"`
	}
	var results *xorm.Rows
	var err error
	if results, err = db.GetEngine(ctx).
		Table("oauth2_grant").
		Where("user_id = ?", uid).
		Join("INNER", "oauth2_application", "application_id = oauth2_application.id").
		Rows(new(joinedOAuth2Grant)); err != nil {
		return nil, err
	}
	defer results.Close()
	grants := make([]*OAuth2Grant, 0)
	for results.Next() {
		joinedGrant := new(joinedOAuth2Grant)
		if err := results.Scan(joinedGrant); err != nil {
			return nil, err
		}
		joinedGrant.Grant.Application = joinedGrant.Application
		grants = append(grants, joinedGrant.Grant)
	}
	return grants, nil
}

// RevokeOAuth2Grant deletes the grant with grantID and userID
func RevokeOAuth2Grant(ctx context.Context, grantID, userID int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{"id": grantID, "user_id": userID}).Delete(&OAuth2Grant{})
	return err
}
