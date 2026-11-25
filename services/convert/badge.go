// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"gitea.dev/models/badges"
	api "gitea.dev/modules/structs"
)

// ToBadge converts a storage badge to the public API badge DTO.
// Color is intentionally omitted until it is added to the API contract.
func ToBadge(badge *badges.Badge) *api.Badge {
	if badge == nil {
		return nil
	}
	return &api.Badge{
		ID:          badge.ID,
		Slug:        badge.Slug,
		Description: badge.Description,
		ImageURL:    badge.ImageURL,
	}
}

// ToBadges converts storage badges to public API badge DTOs.
func ToBadges(list []*badges.Badge) []*api.Badge {
	result := make([]*api.Badge, 0, len(list))
	for _, badge := range list {
		if converted := ToBadge(badge); converted != nil {
			result = append(result, converted)
		}
	}
	return result
}
