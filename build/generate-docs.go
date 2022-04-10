// Copyright 2022 The Gitea Authors. All rights reserved.
// Copyright (c) 2015, Wade Simmons
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// generate-docs generates documents from markdown files in docs/

//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	fmt.Println("Fetch origin branches")
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = curDir
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	releaseBranches, err := fetchAllReleasesBranches(curDir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d branches\n", len(releaseBranches))
	for _, releaseVersion := range releaseBranches {
		if err := genearteOneVersion(curDir, distDir, releaseVersion); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("----", distDir)
}

func fetchAllReleasesBranches(curDir string) ([]string, error) {
	var output bytes.Buffer
	var stderr strings.Builder
	cmd := exec.Command("git", "branch", "-r", "--list", "origin/release/*")
	cmd.Dir = curDir
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v - %v", err, stderr.String())
	}
	var branches []string
	scanner := bufio.NewScanner(&output)

	for scanner.Scan() {
		branch := strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "origin/release/")
		branches = append(branches, branch)
	}

	return branches, nil
}

func genearteOneVersion(curDir, distDir, releaseVersion string) error {
	fmt.Println("Genera branch", releaseVersion)

	distSubDir := filepath.Join(distDir, releaseVersion)

	var stderr strings.Builder
	cmd := exec.Command("git", "switch", "release/"+releaseVersion)
	cmd.Dir = curDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v - %v", err, stderr.String())
	}
	// hugo  $(PUBLIC)
	cmd = exec.Command("hugo", "--cleanDestinationDir", "--destination="+distSubDir)
	cmd.Dir = curDir
	return cmd.Run()
}
