// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// AddOrgMembershipOption add user to organization options
type AddOrgMembershipOption struct {
	Role string `json:"role" binding:"Required"`
}
