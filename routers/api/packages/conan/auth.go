// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conan

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/auth"

	"github.com/golang-jwt/jwt/v4"
)

type Auth struct{}

func (a *Auth) Name() string {
	return "conan"
}

func (a *Auth) Verify(req *http.Request, w http.ResponseWriter, store auth.DataStore, sess auth.SessionStore) *user_model.User {
	token, err := parseAuthorizationToken(req)
	if err != nil {
		log.Trace("parseAuthorizationToken: %v", err)
		return nil
	}

	u, err := user_model.GetUserByID(token.UserID)
	if err != nil {
		log.Error("GetUserByID:  %v", err)
		return nil
	}

	return u
}

type conanClaims struct {
	jwt.RegisteredClaims
	UserID int64
}

func createAuthorizationToken(ctx *context.Context) (string, error) {
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

func parseAuthorizationToken(req *http.Request) (*conanClaims, error) {
	parts := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
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
