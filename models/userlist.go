// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

//UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*User

func (users UserList) getUserIDs() []int64 {
	userIDs := make([]int64, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID) //Considering that user id are unique in the list
	}
	return userIDs
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func (users UserList) IsUserOrgOwner(orgID int64) map[int64]bool {
	results := make(map[int64]bool, len(users))
	//TODO use directly xorm
	for _, u := range users {
		results[u.ID] = u.IsUserOrgOwner(orgID)
	}
	return results
}

// GetTwoFaStatus return state of 2FA enrollement
func (users UserList) GetTwoFaStatus() map[int64]bool {
	results := make(map[int64]bool, len(users))
	//TODO use directly xorm
	for _, u := range users {
		results[u.ID] = u.IsTwoFaEnrolled()
	}
	return results
}
