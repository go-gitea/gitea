// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"

	"github.com/google/go-github/v61/github"
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
