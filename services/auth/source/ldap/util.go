// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

// composeFullName composes a firstname surname or username
func composeFullName(firstname, surname, username string) string {
	switch {
	case firstname == "" && surname == "":
		return username
	case firstname == "":
		return surname
	case surname == "":
		return firstname
	default:
		return firstname + " " + surname
	}
}
