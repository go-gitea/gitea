// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// UserTypeString defines the user type as rendered in API responses, webhook payloads and
// the resulting GitHub Actions event context, where workflows read it as `github.event.sender.type`.
// The values are capitalized to stay compatible with GitHub, unlike VisibilityString and the other
// lowercase API enums. The DB representation is user.UserType (int).
// swagger:enum UserTypeString
type UserTypeString string

const (
	UserTypeStringUser         UserTypeString = "User"
	UserTypeStringOrganization UserTypeString = "Organization"
	UserTypeStringBot          UserTypeString = "Bot"
)
