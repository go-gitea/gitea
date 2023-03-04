// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// AddOrgMembershipOption add user to organization options
type AddOrgMembershipOption struct {
	Role string `json:"role" binding:"Required"`
}
