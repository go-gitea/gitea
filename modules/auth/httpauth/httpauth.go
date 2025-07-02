// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpauth

import (
	"encoding/base64"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

func ParseAuthorizationHeaderBasic(header string) (string, string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 {
		return "", "", false
	}
	if !util.AsciiEqualFold(parts[0], "basic") {
		return "", "", false
	}
	s, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}
	if u, p, ok := strings.Cut(string(s), ":"); ok {
		return u, p, true
	}
	return "", "", false
}

func ParseAuthorizationHeaderBearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 {
		return "", false
	}
	if util.AsciiEqualFold(parts[0], "token") || util.AsciiEqualFold(parts[0], "bearer") {
		return parts[1], true
	}
	return "", false
}
