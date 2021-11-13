// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// AlphaDashDotPattern represents Characters prohibited in a user name (anything except A-Za-z0-9_.-)
	AlphaDashDotPattern = regexp.MustCompile(`[^\w-\.]`)

	// ErrNameEmpty name is empty error
	ErrNameEmpty = errors.New("Name is empty")
)

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
