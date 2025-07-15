// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpauth

import (
	"encoding/base64"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

type BasicAuth struct {
	Username, Password string
}

type BearerToken struct {
	Token string
}

type ParsedAuthorizationHeader struct {
	BasicAuth   *BasicAuth
	BearerToken *BearerToken
}

func ParseAuthorizationHeader(header string) (ret ParsedAuthorizationHeader, _ bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 {
		return ret, false
	}
	if util.AsciiEqualFold(parts[0], "basic") {
		s, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return ret, false
		}
		u, p, ok := strings.Cut(string(s), ":")
		if !ok {
			return ret, false
		}
		ret.BasicAuth = &BasicAuth{Username: u, Password: p}
		return ret, true
	} else if util.AsciiEqualFold(parts[0], "token") || util.AsciiEqualFold(parts[0], "bearer") {
		ret.BearerToken = &BearerToken{Token: parts[1]}
		return ret, true
	}
	return ret, false
}
