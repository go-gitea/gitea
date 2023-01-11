// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"
)

// CORSConfig defines CORS settings
var CORSConfig = struct {
	Enabled          bool
	Scheme           string
	AllowDomain      []string
	AllowSubdomain   bool
	Methods          []string
	MaxAge           time.Duration
	AllowCredentials bool
	Headers          []string
	XFrameOptions    string
}{
	Enabled:       false,
	MaxAge:        10 * time.Minute,
	Headers:       []string{"Content-Type", "User-Agent"},
	XFrameOptions: "SAMEORIGIN",
}
