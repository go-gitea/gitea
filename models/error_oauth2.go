// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "fmt"

// ErrOpenIDConnectInitialize represents a "OpenIDConnectInitialize" kind of error.
type ErrOpenIDConnectInitialize struct {
	OpenIDConnectAutoDiscoveryURL string
	ProviderName                  string
	Cause                         error
}

// IsErrOpenIDConnectInitialize checks if an error is a ExternalLoginUserAlreadyExist.
func IsErrOpenIDConnectInitialize(err error) bool {
	_, ok := err.(ErrOpenIDConnectInitialize)
	return ok
}

func (err ErrOpenIDConnectInitialize) Error() string {
	return fmt.Sprintf("Failed to initialize OpenID Connect Provider with name '%s' with url '%s': %v", err.ProviderName, err.OpenIDConnectAutoDiscoveryURL, err.Cause)
}
