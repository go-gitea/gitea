// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"fmt"
	"mime"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// wellKnownMimeTypesLower comes from Golang's builtin mime package: `builtinTypesLower`, see the comment of DetectWellKnownMimeType
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

// DetectWellKnownMimeType will return the mime-type for a well-known file ext name
// The purpose of this function is to bypass the unstable behavior of Golang's mime.TypeByExtension
// mime.TypeByExtension would use OS's mime-type config to overwrite the well-known types (see its document).
// If the user's OS has incorrect mime-type config, it would make Gitea can not respond a correct Content-Type to browsers.
// For example, if Gitea returns `text/plain` for a `.js` file, the browser couldn't run the JS due to security reasons.
// DetectWellKnownMimeType makes the Content-Type for well-known files stable.
func DetectWellKnownMimeType(ext string) string {
	ext = strings.ToLower(ext)
	return wellKnownMimeTypesLower[ext]
}

type ContentDispositionType string

const (
	ContentDispositionInline     ContentDispositionType = "inline"
	ContentDispositionAttachment ContentDispositionType = "attachment"
)

func needsEncodingRune(b rune) bool {
	return (b < ' ' || b > '~') && b != '\t'
}

// getSafeName replaces all invalid chars in the filename field by underscore
func getSafeName(s string) (_ string, needsEncoding bool) {
	var out strings.Builder
	for _, b := range s {
		if needsEncodingRune(b) {
			needsEncoding = true
			out.WriteRune('_')
		} else {
			out.WriteRune(b)
		}
	}
	return out.String(), needsEncoding
}

// EncodeContentDisposition encodes a correct Content-Disposition Header
func EncodeContentDisposition(t ContentDispositionType, filename string) string {
	safeFilename, needsEncoding := getSafeName(filename)
	result := mime.FormatMediaType(string(t), map[string]string{"filename": safeFilename})
	// No need for the utf8 encoding
	if !needsEncoding {
		return result
	}
	utf8Result := mime.FormatMediaType(string(t), map[string]string{"filename": filename})

	// The mime package might have unexpected results in other go versions
	// Make tests instance fail, otherwise use the default behavior of the go mime package
	if !strings.HasPrefix(result, fmt.Sprintf("%s; filename=", string(t))) || !strings.HasPrefix(utf8Result, fmt.Sprintf("%s; filename*=", string(t))) {
		setting.PanicInDevOrTesting("Unexpected mime package result %s", result)
		return utf8Result
	}

	encodedFileName := strings.TrimPrefix(utf8Result, string(t))
	return result + encodedFileName
}
