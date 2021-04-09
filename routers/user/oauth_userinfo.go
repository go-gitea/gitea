// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"

	"code.gitea.io/gitea/modules/context"
)

// userInfoResponse represents a successful userinfo response
type userInfoResponse struct {
	Sub      string `json:"sub"`
	Name     string `json:"name"`
	Username string `json:"preferred_username"`
	Email    string `json:"email"`
	Picture  string `json:"picture"`
}

// UserInfoOAauth responds with OAuth formatted userinfo
func UserInfoOAuth(ctx *context.Context) {
	user := ctx.User
	response := &userInfoResponse{
		Sub:      fmt.Sprint(user.ID),
		Name:     user.FullName,
		Username: user.Name,
		Email:    user.Email,
		Picture:  user.AvatarLink(),
	}
	ctx.JSON(200, response)
}
