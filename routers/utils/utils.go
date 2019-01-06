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

// IsValidSlackChannel validates a channel name conforms to what slack expects.
// It makes sure a channel name cannot be empty and invalid ( only an # )
func IsValidSlackChannel(channelName string) bool {
	switch len(strings.TrimSpace(channelName)) {
	case 0:
		return false
	case 1:
		// Keep default behaviour where a channel name is still
		// valid without an #
		// But if it contains only an #, it should be regarded as
		// invalid
		if channelName[0] == '#' {
			return false
		}
	}

	return true
}
