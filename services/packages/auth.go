// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/golang-jwt/jwt/v4"
)

type packageClaims struct {
	jwt.RegisteredClaims
	UserID int64
}

func CreateAuthorizationToken(u *user_model.User) (string, error) {
	now := time.Now()

	claims := packageClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID: u.ID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(setting.SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ParseAuthorizationToken(req *http.Request) (int64, error) {
	h := req.Header.Get("Authorization")
	if h == "" {
		return 0, nil
	}

	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		log.Error("split token failed: %s", h)
		return 0, fmt.Errorf("split token failed")
	}

	token, err := jwt.ParseWithClaims(parts[1], &packageClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(setting.SecretKey), nil
	})
	if err != nil {
		return 0, err
	}

	c, ok := token.Claims.(*packageClaims)
	if !token.Valid || !ok {
		return 0, fmt.Errorf("invalid token claim")
	}

	return c.UserID, nil
}
