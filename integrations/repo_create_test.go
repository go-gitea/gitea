// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"net/http"
	"os"
	"testing"

	"code.gitea.io/gitea/integrations/internal/utils"
)

var createRepoFormSample map[string][]string = map[string][]string{
	"UID":         []string{},
	"RepoName":    []string{},
	"Private":     []string{},
	"Description": []string{},
	"AutoInit":    []string{},
	"Gitignores":  []string{},
	"License":     []string{},
	"Readme":      []string{},
}

func repoCreate(t *utils.T) error {
	return utils.GetAndPost("http://:"+ServerHTTPPort+"/repo/create", createRepoFormSample, http.StatusOK)
}

func TestRepoCreate(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", ServerHTTPPort},
		LogFile: os.Stderr,
	}

	if err := utils.New(t, &conf).RunTest(install, signup, repoCreate); err != nil {
		t.Fatal(err)
	}
}
