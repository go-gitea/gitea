// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/golang-jwt/jwt/v5"
)

const idTokenRequestScope = "Actions.IDTokenRequest" // separate from the runtime token, which runners also send to cache servers

type actionsClaims struct {
	jwt.RegisteredClaims
	Scp    string `json:"scp"`
	TaskID int64
	RunID  int64
	JobID  int64
	Ac     string `json:"ac"`
}

type actionsCacheScope struct {
	Scope      string
	Permission actionsCachePermission
}

type actionsCachePermission int

const (
	actionsCachePermissionRead = 1 << iota
	actionsCachePermissionWrite
)

func CreateAuthorizationToken(taskID, runID, jobID int64) (string, error) {
	ac, err := json.Marshal(&[]actionsCacheScope{
		{
			Scope:      "",
			Permission: actionsCachePermissionWrite,
		},
	})
	if err != nil {
		return "", err
	}

	return signActionsClaims(actionsClaims{
		Scp:    fmt.Sprintf("Actions.Results:%d:%d", runID, jobID),
		Ac:     string(ac),
		TaskID: taskID,
		RunID:  runID,
		JobID:  jobID,
	})
}

func signActionsClaims(claims actionsClaims) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(1*time.Hour + setting.Actions.EndlessTaskTimeout)),
		NotBefore: jwt.NewNumericDate(now),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(setting.GetGeneralTokenSigningSecret())
}

func ParseAuthorizationToken(req *http.Request) (int64, error) {
	h := req.Header.Get("Authorization")
	if h == "" {
		return 0, nil
	}

	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		log.Error("split token failed: %s", h)
		return 0, errors.New("split token failed")
	}

	return TokenToTaskID(parts[1])
}

// TokenToTaskID returns the TaskID associated with the provided JWT token
func TokenToTaskID(token string) (int64, error) {
	return taskIDFromToken(token, false)
}

func taskIDFromToken(token string, idTokenRequest bool) (int64, error) {
	parsedToken, err := jwt.ParseWithClaims(token, &actionsClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return setting.GetGeneralTokenSigningSecret(), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := parsedToken.Claims.(*actionsClaims)
	if !parsedToken.Valid || !ok {
		return 0, errors.New("invalid token claim")
	}
	if (claims.Scp == idTokenRequestScope) != idTokenRequest {
		return 0, errors.New("unexpected token scope")
	}

	return claims.TaskID, nil
}
