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
	"Name":   []string{"tester"},
	"Email":  []string{"user1@example.com"},
	"Passwd": []string{"12345678"},
}

func signup(t *utils.T) error {
	var err error
	var r *http.Response

	r, err = http.Get("http://:" + ServerHTTPPort + "/user/sign_up")
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("GET '/user/signup': %s", r.Status)
	}

	r, err = http.PostForm("http://:"+ServerHTTPPort+"/user/sign_up", signupFormSample)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("POST '/user/signup': %s", r.Status)
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

	if err := utils.New(t, &conf).RunTest(install, signup); err != nil {
		t.Fatal(err)
	}
}
