// Copyright 2022 The Gitea Authors. All rights reserved.
// Copyright (c) 2015, Wade Simmons
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// generate-docs generates documents from markdown files in docs/

//go:build ignore
// +build ignore

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
)

func dlThemeFile(dir string) error {
	resp, err := http.Get("https://dl.gitea.io/theme/master.tar.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	themePath := filepath.Join(dir, "theme.tar.gz")
	f, err := os.Create(themePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(f, resp.Body); err != nil {
		return err
	}
	f.Close()

	f2, err := os.Open(themePath)
	if err != nil {
		return err
	}
	defer f2.Close()

	dstPath := filepath.Join(dir, "gitea")

	// gzip read
	gr, err := gzip.NewReader(f2)
	if err != nil {
		return err
	}
	defer gr.Close()

	// tar read
	tr := tar.NewReader(gr)

	// 读取文件
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.FileInfo().IsDir() {
			continue
		}
		if err := os.MkdirAll(filepath.Join(dstPath, path.Dir(h.Name)), os.ModePerm); err != nil {
			return err
		}

		// 打开文件
		fw, err := os.Create(filepath.Join(dstPath, h.Name))
		if err != nil {
			return err
		}
		defer fw.Close()

		// 写文件
		_, err = io.Copy(fw, tr)
		if err != nil {
			return err
		}
		fw.Close()
	}
	return nil
}

func main() {
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Current directory is", curDir)
	tmpDir := os.TempDir()
	distDir, err := os.MkdirTemp(tmpDir, "gitea-docs-dist")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Dist directory is", distDir)

	fmt.Println("Fetching origin branches")
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
	workspaceDir, err := os.MkdirTemp(tmpDir, "gitea-docs-workspace")
	if err != nil {
		log.Fatal(err)
	}

	if err = dlThemeFile(workspaceDir); err != nil {
		log.Fatal(err)
	}

	for _, releaseVersion := range releaseBranches {
		if err := genearteOneVersion(workspaceDir, curDir, distDir, releaseVersion); err != nil {
			log.Fatal(err)
		}
	}
}

var minDocVersion, _ = version.NewVersion("v1.16")

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
		ver, _ := version.NewVersion(branch)
		if ver.LessThan(minDocVersion) {
			continue
		}

		branches = append(branches, branch)
	}

	sort.Slice(branches, func(i, j int) bool {
		a, _ := version.NewVersion(branches[i])
		b, _ := version.NewVersion(branches[j])
		return a.LessThan(b)
	})

	return branches, nil
}

func genearteOneVersion(workspaceDir, gitDir, distDir, releaseVersion string) error {
	curVerDir := filepath.Join(workspaceDir, releaseVersion)
	fmt.Printf("Genera branch %s in %s\n", releaseVersion, curVerDir)

	distSubDir := filepath.Join(distDir, releaseVersion)

	var stderr strings.Builder
	cmd := exec.Command("git", "clone", "-b", "release/"+releaseVersion, gitDir, releaseVersion)
	cmd.Dir = workspaceDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v - %v", err, stderr.String())
	}

	stderr.Reset()
	// hugo  $(PUBLIC)
	cmd = exec.Command("make", `clean`, "trans-copy")
	cmd.Dir = filepath.Join(curVerDir, "docs")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v - %v", err, stderr.String())
	}

	if err := os.MkdirAll(filepath.Join(curVerDir, "docs", "themes", "gitea"), os.ModePerm); err != nil {
		return err
	}

	stderr.Reset()
	cmd = exec.Command("cp", `-r`, filepath.Join(workspaceDir, "gitea"), filepath.Join(curVerDir, "docs", "themes", "gitea"))
	cmd.Dir = filepath.Join(curVerDir, "docs")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v - %v", err, stderr.String())
	}

	stderr.Reset()
	cmd = exec.Command("hugo", `--baseURL="/"`, "--cleanDestinationDir", "--destination="+distSubDir)
	cmd.Dir = filepath.Join(curVerDir, "docs")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v - %v", err, stderr.String())
	}
	return nil
}
