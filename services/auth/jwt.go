// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"

	app_model "code.gitea.io/gitea/models/application"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/httpauth"
)

// Ensure the struct implements the interface.
var (
	_ Method = &JWTAuth{}
)

type JWTAuth struct{}

// Name represents the name of auth method
func (j *JWTAuth) Name() string {
	return "jwt"
}

func verifyJWTToken(ctx context.Context, tokenString string) (*user_model.User, error) {
	claims, err := app_model.ValidateJWTSignature(ctx, tokenString)
	if err != nil {
		return nil, err
	}

	return claims.App.AsUser(), nil
}

func (j *JWTAuth) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	// These paths are not API paths, but we still want to check for tokens because they maybe in the API returned URLs
	detector := newAuthPathDetector(req)
	if !detector.isAPIPath() && !detector.isAttachmentDownload() && !detector.isAuthenticatedTokenRequest() &&
		!detector.isGitRawOrAttachPath() && !detector.isArchivePath() {
		return nil, nil
	}

	// check header token
	if auHead := req.Header.Get("Authorization"); auHead != "" {
		parsed, ok := httpauth.ParseAuthorizationHeader(auHead)
		if ok && parsed.BearerToken != nil {
			return verifyJWTToken(req.Context(), parsed.BearerToken.Token)
		}
	}

	return nil, nil
}
