// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"net/mail"
	"strings"
)

var ErrInvalidCommitTrailerValueSyntax = errors.New("syntax error occurred while parsing a commit trailer value")

// ParseCommitTrailerValueWithAuthor parses a commit trailer value that contains author data.
// Note that it only parses the value and does not consider the trailer key i.e. we just
// parse something like the following:
//
// Foo Bar <foobar@example.com>
func ParseCommitTrailerValueWithAuthor(value string) (name, email string, err error) {
	value = strings.TrimSpace(value)
	if !strings.HasSuffix(value, ">") {
		return "", "", ErrInvalidCommitTrailerValueSyntax
	}

	closedBracketIdx := len(value) - 1
	openBracketIdx := strings.LastIndex(value, "<")
	if openBracketIdx == -1 {
		return "", "", ErrInvalidCommitTrailerValueSyntax
	}

	email = value[openBracketIdx+1 : closedBracketIdx]
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", ErrInvalidCommitTrailerValueSyntax
	}

	name = strings.TrimSpace(value[:openBracketIdx])
	if len(name) == 0 {
		return "", "", ErrInvalidCommitTrailerValueSyntax
	}

	return name, email, nil
}
