// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package db

package user

import "regexp"

var validUsernamePattern = regexp.MustCompile(`^[\da-zA-Z][-.\w]*$`)

// IsValidUsername checks if name is valid
func IsValidUsername(name string) bool {
	return validUsernamePattern.MatchString(name)
}
