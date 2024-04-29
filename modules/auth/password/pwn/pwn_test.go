// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pwn

import (
	"math/rand/v2"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var client = New(WithHTTP(&http.Client{
	Timeout: time.Second * 2,
}))

func TestPassword(t *testing.T) {
	// Check input error
	_, err := client.CheckPassword("", false)
	assert.ErrorIs(t, err, ErrEmptyPassword, "blank input should return ErrEmptyPassword")

	// Should fail
	fail := "password1234"
	count, err := client.CheckPassword(fail, false)
	assert.NotEmpty(t, count, "%s should fail as a password", fail)
	assert.NoError(t, err)

	// Should fail (with padding)
	failPad := "administrator"
	count, err = client.CheckPassword(failPad, true)
	assert.NotEmpty(t, count, "%s should fail as a password", failPad)
	assert.NoError(t, err)

	// Checking for a "good" password isn't going to be perfect, but we can give it a good try
	// with hopefully minimal error. Try five times?
	assert.Condition(t, func() bool {
		for i := 0; i <= 5; i++ {
			count, err = client.CheckPassword(testPassword(), false)
			assert.NoError(t, err)
			if count == 0 {
				return true
			}
		}
		return false
	}, "no generated passwords passed. there is a chance this is a fluke")

	// Again, but with padded responses
	assert.Condition(t, func() bool {
		for i := 0; i <= 5; i++ {
			count, err = client.CheckPassword(testPassword(), true)
			assert.NoError(t, err)
			if count == 0 {
				return true
			}
		}
		return false
	}, "no generated passwords passed. there is a chance this is a fluke")
}

// Credit to https://golangbyexample.com/generate-random-password-golang/
// DO NOT USE THIS FOR AN ACTUAL PASSWORD GENERATOR
var (
	lowerCharSet   = "abcdedfghijklmnopqrst"
	upperCharSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialCharSet = "!@#$%&*"
	numberSet      = "0123456789"
	allCharSet     = lowerCharSet + upperCharSet + specialCharSet + numberSet
)

func testPassword() string {
	var password strings.Builder

	// Set special character
	for i := 0; i < 5; i++ {
		random := rand.IntN(len(specialCharSet))
		password.WriteString(string(specialCharSet[random]))
	}

	// Set numeric
	for i := 0; i < 5; i++ {
		random := rand.IntN(len(numberSet))
		password.WriteString(string(numberSet[random]))
	}

	// Set uppercase
	for i := 0; i < 5; i++ {
		random := rand.IntN(len(upperCharSet))
		password.WriteString(string(upperCharSet[random]))
	}

	for i := 0; i < 5; i++ {
		random := rand.IntN(len(allCharSet))
		password.WriteString(string(allCharSet[random]))
	}
	inRune := []rune(password.String())
	rand.Shuffle(len(inRune), func(i, j int) {
		inRune[i], inRune[j] = inRune[j], inRune[i]
	})
	return string(inRune)
}
