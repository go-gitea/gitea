// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"os"
	"testing"

	"code.gitea.io/gitea/integrations/internal/utils"
)

var signupFormSample map[string][]string = map[string][]string{
	"Name":   {"tester"},
	"Email":  {"user1@example.com"},
	"Passwd": {"12345678"},
}

func signup(t *utils.T) error {
	return utils.GetAndPost("http://:"+ServerHTTPPort+"/user/sign_up", signupFormSample)
}

func TestSignup(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", ServerHTTPPort},
		LogFile: os.Stderr,
	}

	if err := utils.New(t, &conf).RunTest(install, signup); err != nil {
		t.Fatal(err)
	}
}
