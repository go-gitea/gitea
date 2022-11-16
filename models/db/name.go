// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/util"
)

var (
	// ErrNameEmpty name is empty error
	ErrNameEmpty = util.SilentWrap{Message: "name is empty", Err: util.ErrInvalidArgument}

	// AlphaDashDotPattern characters prohibited in a user name (anything except A-Za-z0-9_.-)
	AlphaDashDotPattern = regexp.MustCompile(`[^\w-\.]`)
)

// ErrNameReserved represents a "reserved name" error.
type ErrNameReserved struct {
	Name string
}

// IsErrNameReserved checks if an error is a ErrNameReserved.
func IsErrNameReserved(err error) bool {
	_, ok := err.(ErrNameReserved)
	return ok
}

func (err ErrNameReserved) Error() string {
	return fmt.Sprintf("name is reserved [name: %s]", err.Name)
}

// Unwrap unwraps this as a ErrInvalid err
func (err ErrNameReserved) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrNamePatternNotAllowed represents a "pattern not allowed" error.
type ErrNamePatternNotAllowed struct {
	Pattern string
}

// IsErrNamePatternNotAllowed checks if an error is an ErrNamePatternNotAllowed.
func IsErrNamePatternNotAllowed(err error) bool {
	_, ok := err.(ErrNamePatternNotAllowed)
	return ok
}

func (err ErrNamePatternNotAllowed) Error() string {
	return fmt.Sprintf("name pattern is not allowed [pattern: %s]", err.Pattern)
}

// Unwrap unwraps this as a ErrInvalid err
func (err ErrNamePatternNotAllowed) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrNameCharsNotAllowed represents a "character not allowed in name" error.
type ErrNameCharsNotAllowed struct {
	Name string
}

// IsErrNameCharsNotAllowed checks if an error is an ErrNameCharsNotAllowed.
func IsErrNameCharsNotAllowed(err error) bool {
	_, ok := err.(ErrNameCharsNotAllowed)
	return ok
}

func (err ErrNameCharsNotAllowed) Error() string {
	return fmt.Sprintf("name is invalid [%s]: must be valid alpha or numeric or dash(-_) or dot characters", err.Name)
}

// Unwrap unwraps this as a ErrInvalid err
func (err ErrNameCharsNotAllowed) Unwrap() error {
	return util.ErrInvalidArgument
}

// IsUsableName checks if name is reserved or pattern of name is not allowed
// based on given reserved names and patterns.
// Names are exact match, patterns can be prefix or suffix match with placeholder '*'.
func IsUsableName(names, patterns []string, name string) error {
	name = strings.TrimSpace(strings.ToLower(name))
	if utf8.RuneCountInString(name) == 0 {
		return ErrNameEmpty
	}

	for i := range names {
		if name == names[i] {
			return ErrNameReserved{name}
		}
	}

	for _, pat := range patterns {
		if pat[0] == '*' && strings.HasSuffix(name, pat[1:]) ||
			(pat[len(pat)-1] == '*' && strings.HasPrefix(name, pat[:len(pat)-1])) {
			return ErrNamePatternNotAllowed{pat}
		}
	}

	return nil
}
