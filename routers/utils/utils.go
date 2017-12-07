// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"strings"
)

// RemoveUsernameParameterSuffix returns the username parameter without the (fullname) suffix - leaving just the username
func RemoveUsernameParameterSuffix(name string) string {
	if index := strings.Index(name, " ("); index >= 0 {
		name = name[:index]
	}
	return name
}
