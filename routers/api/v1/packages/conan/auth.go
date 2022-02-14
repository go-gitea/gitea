// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conan

import (
	"fmt"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/golang-jwt/jwt/v4"
)

type conanClaims struct {
	jwt.RegisteredClaims
	UserID int64
}

// CheckAuth is a middleware which handles the custom Conan authorization token
func CheckAuth(ctx *context.APIContext) {
	if ctx.User == nil {
		token, err := parseAuthorizationToken(ctx)
		if err != nil {
			log.Trace("parseAuthorizationToken: %v", err)
			return
		}

		ctx.User, err = user_model.GetUserByIDCtx(ctx, token.UserID)
		if err != nil {
			if !user_model.IsErrUserNotExist(err) {
				log.Error("GetUserByID: %v", err)
			}
			return
		}

		ctx.IsSigned = ctx.User != nil
	}
}

func createAuthorizationToken(ctx *context.APIContext) (string, error) {
	now := time.Now()
	claims := conanClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID: ctx.User.ID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(setting.SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func parseAuthorizationToken(ctx *context.APIContext) (*conanClaims, error) {
	parts := strings.SplitN(ctx.Req.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("no token")
	}

	token, err := jwt.ParseWithClaims(parts[1], &conanClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(setting.SecretKey), nil
	})
	if err != nil {
		return nil, err
	}

	c, ok := token.Claims.(*conanClaims)
	if !token.Valid || !ok {
		return nil, fmt.Errorf("invalid token claim")
	}

	return c, nil
}
