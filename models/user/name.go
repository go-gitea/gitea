// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package db

package user

import (
	"regexp"
)

var (
	validUsernamePattern   = regexp.MustCompile(`^[\da-zA-Z][-.\w]*$`)
	invalidUsernamePattern = regexp.MustCompile(`[-._]{2,}|[-._]$`)
)

// IsValidUsername checks if name is valid
func IsValidUsername(name string) bool {
	// It is difficult to find a single pattern that is both readable and effective,
	// but it's easier to use positive and negative checks.
	return validUsernamePattern.MatchString(name) && !invalidUsernamePattern.MatchString(name)
}
