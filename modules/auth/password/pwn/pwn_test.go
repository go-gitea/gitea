// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pwn

import (
	"errors"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var client = New(WithHTTP(&http.Client{
	Timeout: time.Second * 2,
}))

func TestMain(m *testing.M) {
	rand.Seed(time.Now().Unix())
	os.Exit(m.Run())
}

func TestPassword(t *testing.T) {
	// Check input error
	_, err := client.CheckPassword("", false)
	if err == nil {
		t.Log("blank input should return an error")
		t.Fail()
	}
	if !errors.Is(err, ErrEmptyPassword) {
		t.Log("blank input should return ErrEmptyPassword")
		t.Fail()
	}

	// Should fail
	fail := "password1234"
	count, err := client.CheckPassword(fail, false)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if count == 0 {
		t.Logf("%s should fail as a password\n", fail)
		t.Fail()
	}

	// Should fail (with padding)
	failPad := "administrator"
	count, err = client.CheckPassword(failPad, true)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if count == 0 {
		t.Logf("%s should fail as a password\n", failPad)
		t.Fail()
	}

	// Checking for a "good" password isn't going to be perfect, but we can give it a good try
	// with hopefully minimal error. Try five times?
	var good bool
	var pw string
	for idx := 0; idx <= 5; idx++ {
		pw = testPassword()
		count, err = client.CheckPassword(pw, false)
		if err != nil {
			t.Log(err)
			t.Fail()
		}
		if count == 0 {
			good = true
			break
		}
	}
	if !good {
		t.Log("no generated passwords passed. there is a chance this is a fluke")
		t.Fail()
	}

	// Again, but with padded responses
	good = false
	for idx := 0; idx <= 5; idx++ {
		pw = testPassword()
		count, err = client.CheckPassword(pw, true)
		if err != nil {
			t.Log(err)
			t.Fail()
		}
		if count == 0 {
			good = true
			break
		}
	}
	if !good {
		t.Log("no generated passwords passed. there is a chance this is a fluke")
		t.Fail()
	}
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
		random := rand.Intn(len(specialCharSet))
		password.WriteString(string(specialCharSet[random]))
	}

	// Set numeric
	for i := 0; i < 5; i++ {
		random := rand.Intn(len(numberSet))
		password.WriteString(string(numberSet[random]))
	}

	// Set uppercase
	for i := 0; i < 5; i++ {
		random := rand.Intn(len(upperCharSet))
		password.WriteString(string(upperCharSet[random]))
	}

	for i := 0; i < 5; i++ {
		random := rand.Intn(len(allCharSet))
		password.WriteString(string(allCharSet[random]))
	}
	inRune := []rune(password.String())
	rand.Shuffle(len(inRune), func(i, j int) {
		inRune[i], inRune[j] = inRune[j], inRune[i]
	})
	return string(inRune)
}
