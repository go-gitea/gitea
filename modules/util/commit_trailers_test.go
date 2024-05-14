// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommitTrailerValueWithAuthor(t *testing.T) {
	cases := []struct {
		input         string
		shouldBeError bool
		expectedName  string
		expectedEmail string
	}{
		{"Foo Bar <foobar@example.com", true, "", ""},
		{"Foo Bar foobar@example.com>", true, "", ""},
		{"Foo Bar <>", true, "", ""},
		{"Foo Bar <invalid-email-address>", true, "", ""},
		{"<foobar@example.com>", true, "", ""},
		{"    <foobar@example.com>", true, "", ""},
		{"Foo Bar <foobar@example.com>", false, "Foo Bar", "foobar@example.com"},
		{"   Foo Bar    <foobar@example.com>", false, "Foo Bar", "foobar@example.com"},
		// Account for edge case where name contains an open bracket.
		{"   Foo < Bar    <foobar@example.com>", false, "Foo < Bar", "foobar@example.com"},
	}

	for n, c := range cases {
		name, email, err := ParseCommitTrailerValueWithAuthor(c.input)
		if c.shouldBeError {
			assert.Error(t, err, "case %d should be a syntax error", n)
		} else {
			assert.Equal(t, c.expectedName, name, "case %d should have correct name", n)
			assert.Equal(t, c.expectedEmail, email, "case %d should have correct email", n)
		}
	}
}
