// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"errors"
	"net/url"
	"strings"
)

// ErrInvalidCloneAddr tell us a remote address is invalid
var ErrInvalidCloneAddr = errors.New("remote address is invalid")

// ParseRemoteAddr checks if given remote address is valid,
// and returns composed URL with needed username and password.
func ParseRemoteAddr(remoteAddr, authUsername, authPassword string) (string, error) {
	remoteAddr = strings.TrimSpace(remoteAddr)
	// Remote address can be HTTP/HTTPS/Git URL or local path.
	if strings.HasPrefix(remoteAddr, "http://") ||
		strings.HasPrefix(remoteAddr, "https://") ||
		strings.HasPrefix(remoteAddr, "git://") {
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", ErrInvalidCloneAddr
		}
		if len(authUsername)+len(authPassword) > 0 {
			u.User = url.UserPassword(authUsername, authPassword)
		}
		remoteAddr = u.String()
	}

	return remoteAddr, nil
}

// RemoteAddr use ParseRemoteAddr to validate an address
func RemoteAddr(remoteAddr, authUsername, authPassword string) bool {
	_, err := ParseRemoteAddr(remoteAddr, authUsername, authPassword)
	return err == nil
}
