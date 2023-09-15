// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
)

// UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*User //revive:disable-line:exported

// GetUserIDs returns a slice of user's id
func (users UserList) GetUserIDs() []int64 {
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID) // Considering that user id are unique in the list
	}
	return userIDs
}

// GetTwoFaStatus return state of 2FA enrollement
func (users UserList) GetTwoFaStatus(ctx context.Context) map[int64]bool {
	results := make(map[int64]bool, len(users))
	for _, user := range users {
		results[user.ID] = false // Set default to false
	}

	if tokenMaps, err := users.loadTwoFactorStatus(ctx); err == nil {
		for _, token := range tokenMaps {
			results[token.UID] = true
		}
	}

	if ids, err := users.userIDsWithWebAuthn(ctx); err == nil {
		for _, id := range ids {
			results[id] = true
		}
	}

	return results
}

func (users UserList) loadTwoFactorStatus(ctx context.Context) (map[int64]*auth.TwoFactor, error) {
	if len(users) == 0 {
		return nil, nil
	}

	userIDs := users.GetUserIDs()
	tokenMaps := make(map[int64]*auth.TwoFactor, len(userIDs))
	if err := db.GetEngine(ctx).In("uid", userIDs).Find(&tokenMaps); err != nil {
		return nil, fmt.Errorf("find two factor: %w", err)
	}
	return tokenMaps, nil
}

func (users UserList) userIDsWithWebAuthn(ctx context.Context) ([]int64, error) {
	if len(users) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(users))
	if err := db.GetEngine(ctx).Table(new(auth.WebAuthnCredential)).In("user_id", users.GetUserIDs()).Select("user_id").Distinct("user_id").Find(&ids); err != nil {
		return nil, fmt.Errorf("find two factor: %w", err)
	}
	return ids, nil
}

// GetUsersByIDs returns all resolved users from a list of Ids.
func GetUsersByIDs(ctx context.Context, ids []int64) (UserList, error) {
	ous := make([]*User, 0, len(ids))
	if len(ids) == 0 {
		return ous, nil
	}
	err := db.GetEngine(ctx).In("id", ids).
		Asc("name").
		Find(&ous)
	return ous, err
}
