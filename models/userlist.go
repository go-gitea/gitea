// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

//UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*User

//TODO paginate

// IsPublicMember returns true if user public his/her membership in given organization.
func (ul UserList) IsPublicMember(orgID int64) []bool {
	results := make([]bool, len(ul))
	//TODO use directly xorm
	for i, u := range ul {
		results[i] = u.IsPublicMember(orgID)
	}
	return results
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func (ul UserList) IsUserOrgOwner(orgID int64) []bool {
	results := make([]bool, len(ul))
	//TODO use directly xorm
	for i, u := range ul {
		results[i] = u.IsUserOrgOwner(orgID)
	}
	return results
}

// IsTwoFaEnrolled return state of 2FA enrollement
func (ul UserList) IsTwoFaEnrolled() []bool {
	results := make([]bool, len(ul))
	//TODO use directly xorm
	for i, u := range ul {
		results[i] = u.IsTwoFaEnrolled()
	}
	return results
}
