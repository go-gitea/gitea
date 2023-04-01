// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

// composeFullName composes a firstname surname or username
func composeFullName(firstname, surname, username string) string {
	switch {
	case len(firstname) == 0 && len(surname) == 0:
		return username
	case len(firstname) == 0:
		return surname
	case len(surname) == 0:
		return firstname
	default:
		return firstname + " " + surname
	}
}
