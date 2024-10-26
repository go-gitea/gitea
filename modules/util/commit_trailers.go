// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"net/mail"
)

var ErrInvalidCommitTrailerValueSyntax = errors.New("syntax error occurred while parsing a commit trailer value")

// ParseCommitTrailerValueWithAuthor parses a commit trailer value that contains author data.
// Note that it only parses the value and does not consider the trailer key i.e. we just
// parse something like the following:
//
// Foo Bar <foobar@example.com>
func ParseCommitTrailerValueWithAuthor(value string) (name, email string, err error) {
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return name, email, err
	}

	if addr.Name == "" {
		return name, email, errors.New("commit trailer missing name")
	}

	name = addr.Name
	email = addr.Address

	return name, email, nil
}
