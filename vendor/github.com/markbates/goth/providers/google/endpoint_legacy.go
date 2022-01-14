//go:build !go1.9
// +build !go1.9

package google

import (
	"golang.org/x/oauth2"
)

// Endpoint is Google's OAuth 2.0 endpoint.
var Endpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://accounts.google.com/o/oauth2/token",
}
