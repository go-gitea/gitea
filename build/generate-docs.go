// Copyright 2022 The Gitea Authors. All rights reserved.
// Copyright (c) 2015, Wade Simmons
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// generate-docs generates documents from markdown files in docs/

//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
)

func main() {
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Current Directory is", curDir)

	tmpDir := os.TempDir()
	distDir, err := os.MkdirTemp(tmpDir, "gitea-docs")
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = curDir
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	releaseBranches, err := fetchAllReleasesBranches(curDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, releaseVersion := range releaseBranches {
		if err := genearteOneVersion(curDir, distDir, releaseVersion); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("----", distDir)
}

func fetchAllReleasesBranches(curDir string) ([]string, error) {
	git.NewCommand("branch", "-r", "--list 'origin/release/*'").Run
	cmd := exec.Command("git")
	cmd.Dir = curDir
	cmd.Run()
	repo, err := git.OpenRepository(curDir)
	if err != nil {
		return nil, err
	}

	branches, _, err := repo.GetBranches(0, 0)
	return branches, err
}

func genearteOneVersion(curDir, distDir, releaseVersion string) error {
	distSubDir := filepath.Join(distDir, releaseVersion)

	exec.Command("git", "switch", releaseVersion)
	// hugo  $(PUBLIC)
	cmd := exec.Command("hugo", "--cleanDestinationDir", "--destination="+distSubDir)
	cmd.Dir = curDir
	return cmd.Run()
}
