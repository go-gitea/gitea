// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"slices"
	"strconv"

	"gitea.dev/models/user"
)

type SearchUserInfo struct {
	UserID     int64  `json:"user_id"`
	UserName   string `json:"username"`
	AvatarLink string `json:"avatar_link"`
	FullName   string `json:"full_name"`
}

type SearchUserResponse struct {
	Results []*SearchUserInfo `json:"results"`
}

func MakeSelfOnTop(doer *user.User, users []*user.User) []*user.User {
	if doer == nil {
		return users
	}
	idx := slices.IndexFunc(users, func(u *user.User) bool {
		return u.ID == doer.ID
	})
	if idx > 0 {
		newUsers := make([]*user.User, len(users))
		newUsers[0] = users[idx]
		copy(newUsers[1:], users[:idx])
		copy(newUsers[idx+1:], users[idx+1:])
		return newUsers
	}
	return users
}

func ToSearchUserResponse(ctx context.Context, self *user.User, users []*user.User) (ret *SearchUserResponse) {
	infos := MakeSelfOnTop(self, users)
	resp := &SearchUserResponse{Results: make([]*SearchUserInfo, len(infos))}
	for i, u := range infos {
		resp.Results[i] = &SearchUserInfo{UserID: u.ID, UserName: u.Name, AvatarLink: u.AvatarLink(ctx)}
		resp.Results[i].FullName = u.FullName
	}
	return resp
}

// GetPossibleDoerUserIDByName maps an issue filter username or legacy user ID to an ID, Ghost if unknown, no permission check as issue search enforces visibility
func GetPossibleDoerUserIDByName(ctx context.Context, name string) string {
	if name == "" {
		return ""
	}
	ids, err := user.GetDoerUserIDsByNamesWithSystem(ctx, []string{name})
	if err != nil {
		if id, err := strconv.ParseInt(name, 10, 64); err == nil {
			return strconv.FormatInt(id, 10)
		}
		return strconv.FormatInt(user.GhostUserID, 10)
	}
	return strconv.FormatInt(ids[0], 10)
}
