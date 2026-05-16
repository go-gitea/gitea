// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"mime"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

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

func EncodeContentDispositionAttachment(filename string) string {
	return encodeContentDisposition(ContentDispositionAttachment, filename)
}

func EncodeContentDispositionInline(filename string) string {
	return encodeContentDisposition(ContentDispositionInline, filename)
}

// encodeContentDisposition encodes a correct Content-Disposition Header
func encodeContentDisposition(t ContentDispositionType, filename string) string {
	safeFilename, needsEncoding := getSafeName(filename)
	result := mime.FormatMediaType(string(t), map[string]string{"filename": safeFilename})
	// No need for the utf8 encoding
	if !needsEncoding {
		return result
	}
	utf8Result := mime.FormatMediaType(string(t), map[string]string{"filename": filename})

	// The mime package might have unexpected results in other go versions
	// Make tests instance fail, otherwise use the default behavior of the go mime package
	if !strings.HasPrefix(result, string(t)+"; filename=") || !strings.HasPrefix(utf8Result, string(t)+"; filename*=") {
		setting.PanicInDevOrTesting("Unexpected mime package result %s", result)
		return utf8Result
	}

	encodedFileName := strings.TrimPrefix(utf8Result, string(t))
	return result + encodedFileName
}
