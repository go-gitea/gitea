// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"strconv"
	"strings"

	"gitea.dev/models/auth"
)

type ExtDoerData interface {
	EncodeToString() string
	DecodeFromString(string) error
}

type extDoerGiteaActions struct {
	TaskID int64
}

var _ ExtDoerData = (*extDoerGiteaActions)(nil)

func (e *extDoerGiteaActions) EncodeToString() string {
	return "gitea-actions:" + strconv.FormatInt(e.TaskID, 10)
}

func (e *extDoerGiteaActions) DecodeFromString(s string) (err error) {
	idStr, _ := strings.CutPrefix(s, "gitea-actions:")
	e.TaskID, err = strconv.ParseInt(idStr, 10, 64)
	return err
}

type extDoerDeployKey struct {
	DeployKeyID int64
}

var _ ExtDoerData = (*extDoerDeployKey)(nil)

func (e *extDoerDeployKey) EncodeToString() string {
	return "deploy-key:" + strconv.FormatInt(e.DeployKeyID, 10)
}

func (e *extDoerDeployKey) DecodeFromString(s string) (err error) {
	idStr, _ := strings.CutPrefix(s, "deploy-key:")
	e.DeployKeyID, err = strconv.ParseInt(idStr, 10, 64)
	return err
}

const (
	CredentialAccessToken = "access-token"
	CredentialOAuth2Grant = "oauth2-grant"
)

type extDoerToken struct {
	credential string // empty for package registry tokens
	scope      auth.AccessTokenScope
	publicOnly bool
}

var _ ExtDoerData = (*extDoerToken)(nil)

func NewTokenExtDoerData(credentialKind string, credentialID int64, scope auth.AccessTokenScope) ExtDoerData {
	publicOnly, err := scope.PublicOnly()
	e := &extDoerToken{scope: scope, publicOnly: publicOnly || err != nil} // an unparsable scope fails closed
	if credentialKind != "" {
		e.credential = credentialKind + ":" + strconv.FormatInt(credentialID, 10)
	}
	return e
}

func (e *extDoerToken) EncodeToString() string {
	return e.credential + "|" + string(e.scope)
}

func (e *extDoerToken) DecodeFromString(s string) error {
	credential, scope, ok := strings.Cut(s, "|")
	if !ok {
		return errors.New("invalid token doer data")
	}
	publicOnly, err := auth.AccessTokenScope(scope).PublicOnly()
	if err != nil {
		return err
	}
	e.credential, e.scope, e.publicOnly = credential, auth.AccessTokenScope(scope), publicOnly
	return nil
}

func getDoerToken(u *User) *extDoerToken {
	if u == nil {
		return nil
	}
	token, _ := u.ExtDoerData.(*extDoerToken)
	return token
}

func GetDoerTokenScope(u *User) (auth.AccessTokenScope, bool) {
	if token := getDoerToken(u); token != nil {
		return token.scope, true
	}
	return "", false
}

func IsPublicOnlyDoer(u *User) bool {
	token := getDoerToken(u)
	return token != nil && token.publicOnly
}

// DoerTokenAllowsOwner reports whether the doer's token may reach the owner at all
func DoerTokenAllowsOwner(doer, owner *User) bool {
	return !IsPublicOnlyDoer(doer) || owner.Visibility.IsPublic()
}

// GetDoerCredential names the credential the doer acted with, for audit
func GetDoerCredential(u *User) string {
	if u == nil || u.ExtDoerData == nil {
		return ""
	}
	if token := getDoerToken(u); token != nil {
		return token.credential
	}
	return u.ExtDoerData.EncodeToString()
}
