// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSanitizedError(t *testing.T) {
	err := errors.New("error while secret on test")
	err2 := NewSanitizedError(err)
	assert.Equal(t, err.Error(), err2.Error())

	var cases = []struct {
		input    error
		oldnew   []string
		expected string
	}{
		// case 0
		{
			errors.New("error while secret on test"),
			[]string{"secret", "replaced"},
			"error while replaced on test",
		},
		// case 1
		{
			errors.New("error while sec-ret on test"),
			[]string{"secret", "replaced"},
			"error while sec-ret on test",
		},
	}

	for n, c := range cases {
		err := NewSanitizedError(c.input, c.oldnew...)

		assert.Equal(t, c.expected, err.Error(), "case %d: error should match", n)
	}
}

func TestNewStringURLSanitizer(t *testing.T) {
	var cases = []struct {
		input       string
		placeholder bool
		expected    string
	}{
		// case 0
		{
			"https://github.com/go-gitea/test_repo.git",
			true,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 1
		{
			"https://github.com/go-gitea/test_repo.git",
			false,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 2
		{
			"https://mytoken@github.com/go-gitea/test_repo.git",
			true,
			"https://" + userPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		// case 3
		{
			"https://mytoken@github.com/go-gitea/test_repo.git",
			false,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 4
		{
			"https://user:password@github.com/go-gitea/test_repo.git",
			true,
			"https://" + userPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		// case 5
		{
			"https://user:password@github.com/go-gitea/test_repo.git",
			false,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 6
		{
			"https://gi\nthub.com/go-gitea/test_repo.git",
			false,
			unparsableURL,
		},
	}

	for n, c := range cases {
		// uses NewURLSanitizer internally
		result := NewStringURLSanitizer(c.input, c.placeholder).Replace(c.input)

		assert.Equal(t, c.expected, result, "case %d: error should match", n)
	}
}

func TestNewStringURLSanitizedError(t *testing.T) {
	var cases = []struct {
		input       string
		placeholder bool
		expected    string
	}{
		// case 0
		{
			"https://github.com/go-gitea/test_repo.git",
			true,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 1
		{
			"https://github.com/go-gitea/test_repo.git",
			false,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 2
		{
			"https://mytoken@github.com/go-gitea/test_repo.git",
			true,
			"https://" + userPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		// case 3
		{
			"https://mytoken@github.com/go-gitea/test_repo.git",
			false,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 4
		{
			"https://user:password@github.com/go-gitea/test_repo.git",
			true,
			"https://" + userPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		// case 5
		{
			"https://user:password@github.com/go-gitea/test_repo.git",
			false,
			"https://github.com/go-gitea/test_repo.git",
		},
		// case 6
		{
			"https://gi\nthub.com/go-gitea/test_repo.git",
			false,
			unparsableURL,
		},
	}

	encloseText := func(input string) string {
		return "test " + input + " test"
	}

	for n, c := range cases {
		err := errors.New(encloseText(c.input))

		result := NewStringURLSanitizedError(err, c.input, c.placeholder)

		assert.Equal(t, encloseText(c.expected), result.Error(), "case %d: error should match", n)
	}
}
