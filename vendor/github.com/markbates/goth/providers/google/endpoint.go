//go:build go1.9
// +build go1.9

package google

import (
	goog "golang.org/x/oauth2/google"
)

// Endpoint is Google's OAuth 2.0 endpoint.
var Endpoint = goog.Endpoint
