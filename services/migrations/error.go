// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"
	"net/http"

	"gitea.dev/modules/git/gitcmd"

	"github.com/google/go-github/v89/github"
)

// ErrRepoNotCreated returns the error that repository not created
var ErrRepoNotCreated = errors.New("repository is not created yet")

// IsRateLimitError returns true if the err is github.RateLimitError
func IsRateLimitError(err error) bool {
	_, ok := err.(*github.RateLimitError)
	return ok
}

// IsTwoFactorAuthError returns true if the err is github.TwoFactorAuthError
func IsTwoFactorAuthError(err error) bool {
	_, ok := err.(*github.TwoFactorAuthError)
	return ok
}

// IsAuthenticationError returns true if the remote rejected the credentials, over git or over its HTTP API
func IsAuthenticationError(err error) bool {
	if gitcmd.IsStderr(err, gitcmd.StderrAuthenticationFailed) || gitcmd.IsStderr(err, gitcmd.StderrCouldNotReadUsername) {
		return true
	}
	githubErr, ok := errors.AsType[*github.ErrorResponse](err)
	return ok && githubErr.Response != nil && githubErr.Response.StatusCode == http.StatusUnauthorized
}
