// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/golang-jwt/jwt/v5"
)

type packageClaims struct {
	jwt.RegisteredClaims
	PackageMeta
}
type PackageMeta struct {
	UserID int64
	Scope  auth_model.AccessTokenScope
}

func CreateAuthorizationToken(u *user_model.User, packageScope auth_model.AccessTokenScope) (string, error) {
	now := time.Now()

	claims := packageClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
		PackageMeta: PackageMeta{
			UserID: u.ID,
			Scope:  packageScope,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(setting.GetGeneralTokenSigningSecret())
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ParseAuthorizationRequest(req *http.Request) (*PackageMeta, error) {
	h := req.Header.Get("Authorization")
	if h == "" {
		return nil, nil
	}

	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		log.Error("split token failed: %s", h)
		return nil, fmt.Errorf("split token failed")
	}

	return ParseAuthorizationToken(parts[1])
}

func ParseAuthorizationToken(tokenStr string) (*PackageMeta, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &packageClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return setting.GetGeneralTokenSigningSecret(), nil
	})
	if err != nil {
		return nil, err
	}

	c, ok := token.Claims.(*packageClaims)
	if !token.Valid || !ok {
		return nil, fmt.Errorf("invalid token claim")
	}

	return &c.PackageMeta, nil
}
