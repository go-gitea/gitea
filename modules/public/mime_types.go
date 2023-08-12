// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import "strings"

// wellKnownMimeTypesLower comes from Golang's builtin mime package: `builtinTypesLower`, see the comment of detectWellKnownMimeType
var wellKnownMimeTypesLower = map[string]string{
	".avif": "image/avif",
	".css":  "text/css; charset=utf-8",
	".gif":  "image/gif",
	".htm":  "text/html; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json",
	".mjs":  "text/javascript; charset=utf-8",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".wasm": "application/wasm",
	".webp": "image/webp",
	".xml":  "text/xml; charset=utf-8",

	// well, there are some types missing from the builtin list
	".txt": "text/plain; charset=utf-8",
}

// detectWellKnownMimeType will return the mime-type for a well-known file ext name
// The purpose of this function is to bypass the unstable behavior of Golang's mime.TypeByExtension
// mime.TypeByExtension would use OS's mime-type config to overwrite the well-known types (see its document).
// If the user's OS has incorrect mime-type config, it would make Gitea can not respond a correct Content-Type to browsers.
// For example, if Gitea returns `text/plain` for a `.js` file, the browser couldn't run the JS due to security reasons.
// detectWellKnownMimeType makes the Content-Type for well-known files stable.
func detectWellKnownMimeType(ext string) string {
	ext = strings.ToLower(ext)
	return wellKnownMimeTypesLower[ext]
}
