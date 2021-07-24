// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"net/url"
	"strings"
)

const userPlaceholder = "sanitized-credential"
const unparsableURL = "(unparsable url)"

type sanitizedError struct {
	err      error
	replacer *strings.Replacer
}

func (err sanitizedError) Error() string {
	return err.replacer.Replace(err.err.Error())
}

// NewSanitizedError wraps an error and replaces all old, new string pairs in the message text.
func NewSanitizedError(err error, oldnew ...string) error {
	return sanitizedError{err: err, replacer: strings.NewReplacer(oldnew...)}
}

// NewURLSanitizedError wraps an error and replaces the url credential or removes them.
func NewURLSanitizedError(err error, u *url.URL, usePlaceholder bool) error {
	return sanitizedError{err: err, replacer: NewURLSanitizer(u, usePlaceholder)}
}

// NewStringURLSanitizedError wraps an error and replaces the url credential or removes them.
// If the url can't get parsed it gets replaced with a placeholder string.
func NewStringURLSanitizedError(err error, unsanitizedURL string, usePlaceholder bool) error {
	return sanitizedError{err: err, replacer: NewStringURLSanitizer(unsanitizedURL, usePlaceholder)}
}

// NewURLSanitizer creates a replacer for the url with the credential sanitized or removed.
func NewURLSanitizer(u *url.URL, usePlaceholder bool) *strings.Replacer {
	old := u.String()

	if u.User != nil && usePlaceholder {
		u.User = url.User(userPlaceholder)
	} else {
		u.User = nil
	}
	return strings.NewReplacer(old, u.String())
}

// NewStringURLSanitizer creates a replacer for the url with the credential sanitized or removed.
// If the url can't get parsed it gets replaced with a placeholder string
func NewStringURLSanitizer(unsanitizedURL string, usePlaceholder bool) *strings.Replacer {
	u, err := url.Parse(unsanitizedURL)
	if err != nil {
		// don't log the error, since it might contain unsanitized URL.
		return strings.NewReplacer(unsanitizedURL, unparsableURL)
	}
	return NewURLSanitizer(u, usePlaceholder)
}
