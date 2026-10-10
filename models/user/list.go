// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	"gitea.dev/models/auth"
	"gitea.dev/models/badges"
	"gitea.dev/models/db"
)

// UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*User //revive:disable-line:exported

// LoadBadges loads achievement badges for all users in the list.
func (users UserList) LoadBadges(ctx context.Context) error {
	if len(users) == 0 {
		return nil
	}
	userIDs := users.GetUserIDs()
	for _, user := range users {
		if user.Badges == nil {
			user.Badges = make([]*badges.Badge, 0)
		}
	}
	var userBadges []UserBadge
	if err := db.GetEngine(ctx).Table("user_badge").In("user_id", userIDs).Find(&userBadges); err != nil {
		return err
	}
	badgeIDs := make([]int64, 0, len(userBadges))
	for _, ub := range userBadges {
		badgeIDs = append(badgeIDs, ub.BadgeID)
	}
	if len(badgeIDs) == 0 {
		return nil
	}
	badgeList := make([]*badges.Badge, 0, len(badgeIDs))
	if err := db.GetEngine(ctx).Table("badge").In("id", badgeIDs).Find(&badgeList); err != nil {
		return err
	}
	badgeMap := make(map[int64]*badges.Badge, len(badgeList))
	for _, badge := range badgeList {
		badgeMap[badge.ID] = badge
	}
	userMap := make(map[int64]*User, len(users))
	for _, user := range users {
		userMap[user.ID] = user
	}
	for _, ub := range userBadges {
		if user, ok := userMap[ub.UserID]; ok {
			if badge, ok := badgeMap[ub.BadgeID]; ok {
				user.Badges = append(user.Badges, badge)
			}
		}
	}
	return nil
}

// GetUserIDs returns a slice of user's id
func (users UserList) GetUserIDs() []int64 {
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID) // Considering that user id are unique in the list
	}
	return userIDs
}

// GetTwoFaStatus return state of 2FA enrollment
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
		return nil, nil //nolint:nilnil // returns nil when there are no users
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
