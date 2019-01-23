// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// OptionalBool a boolean that can be "null"
type OptionalBool byte

const (
	// OptionalBoolNone a "null" boolean value
	OptionalBoolNone = iota
	// OptionalBoolTrue a "true" boolean value
	OptionalBoolTrue
	// OptionalBoolFalse a "false" boolean value
	OptionalBoolFalse
)

// IsTrue return true if equal to OptionalBoolTrue
func (o OptionalBool) IsTrue() bool {
	return o == OptionalBoolTrue
}

// IsFalse return true if equal to OptionalBoolFalse
func (o OptionalBool) IsFalse() bool {
	return o == OptionalBoolFalse
}

// IsNone return true if equal to OptionalBoolNone
func (o OptionalBool) IsNone() bool {
	return o == OptionalBoolNone
}

// OptionalBoolOf get the corresponding OptionalBool of a bool
func OptionalBoolOf(b bool) OptionalBool {
	if b {
		return OptionalBoolTrue
	}
	return OptionalBoolFalse
}

// Max max of two ints
func Max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// URLJoin joins url components, like path.Join, but preserving contents
func URLJoin(base string, elems ...string) string {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		log.Error(4, "URLJoin: Invalid base URL %s", base)
		return ""
	}
	joinedPath := path.Join(elems...)
	argURL, err := url.Parse(joinedPath)
	if err != nil {
		log.Error(4, "URLJoin: Invalid arg %s", joinedPath)
		return ""
	}
	joinedURL := baseURL.ResolveReference(argURL).String()
	if !baseURL.IsAbs() && !strings.HasPrefix(base, "/") {
		return joinedURL[1:] // Removing leading '/' if needed
	}
	return joinedURL
}

// IsExternalURL checks if rawURL points to an external URL like http://example.com
func IsExternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	if len(parsed.Host) != 0 && strings.Replace(parsed.Host, "www.", "", 1) != strings.Replace(setting.Domain, "www.", "", 1) {
		return true
	}
	return false
}

// Min min of two ints
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// IsEmptyString checks if the provided string is empty
func IsEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}
