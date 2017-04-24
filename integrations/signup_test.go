// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"code.gitea.io/gitea/integrations/internal/utils"
)

var signupFormSample map[string][]string = map[string][]string{
	"user_name": {"tester"},
	"email":     {"user1@example.com"},
	"password":  {"12345678"},
	"retype":    {"12345678"},
}

var loginFormSample map[string][]string = map[string][]string{
	"user_name": {"tester"},
	"password":  {"12345678"},
}

var WrongLoginFormSample map[string][]string = map[string][]string{
	"user_name": {"tester"},
	"password":  {"22345678"},
}

func signup(t *utils.T) error {
	r, err := http.PostForm("http://:"+ServerHTTPPort+"/user/sign_up", signupFormSample)
	if err != nil {
		return err
	}
	if r.Request.URL.Path != "/user/login" {
		return fmt.Errorf("Unexpected URL of the redirected request: %s", r.Request.URL.Path)
	}
	return nil
}

func wrongLogin(t *utils.T) error {
	r, err := http.PostForm("http://:"+ServerHTTPPort+"/user/login", WrongLoginFormSample)
	if err != nil {
		return err
	}
	if r.Request.URL.Path != "/user/login" {
		return fmt.Errorf("Unexpected URL of the redirected request: %s", r.Request.URL.Path)
	}
	return nil
}

func login(t *utils.T) error {
	r, err := http.PostForm("http://:"+ServerHTTPPort+"/user/login", loginFormSample)
	if err != nil {
		return err
	}
	if r.Request.URL.Path != "/" {
		return fmt.Errorf("Unexpected URL of the redirected request: %s", r.Request.URL.Path)
	}
	return nil
}

func TestSignup(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", ServerHTTPPort},
		LogFile: os.Stderr,
	}

	if err := utils.New(t, &conf).RunTest(install, signup, wrongLogin, login); err != nil {
		t.Fatal(err)
	}
}
