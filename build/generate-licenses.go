// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/build/license"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

func main() {
	var (
		prefix         = "gitea-licenses"
		url            = "https://api.github.com/repos/spdx/license-list-data/tarball"
		githubApiToken = ""
		githubUsername = ""
		destination    = ""
	)

	flag.StringVar(&destination, "dest", "options/license/", "destination for the licenses")
	flag.StringVar(&githubUsername, "username", "", "github username")
	flag.StringVar(&githubApiToken, "token", "", "github api token")
	flag.Parse()

	file, err := os.CreateTemp(os.TempDir(), prefix)
	if err != nil {
		log.Fatalf("Failed to create temp file. %s", err)
	}

	defer util.Remove(file.Name())

	if err := os.RemoveAll(destination); err != nil {
		log.Fatalf("Cannot clean destination folder: %v", err)
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		log.Fatalf("Cannot create destination: %v", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	if len(githubApiToken) > 0 && len(githubUsername) > 0 {
		req.SetBasicAuth(githubUsername, githubApiToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	defer resp.Body.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		log.Fatalf("Failed to copy archive to file. %s", err)
	}

	if _, err := file.Seek(0, 0); err != nil {
		log.Fatalf("Failed to reset seek on archive. %s", err)
	}

	gz, err := gzip.NewReader(file)
	if err != nil {
		log.Fatalf("Failed to gunzip the archive. %s", err)
	}

	tr := tar.NewReader(gz)
	aliasesFiles := make(map[string][]string)
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("Failed to iterate archive. %s", err)
		}

		if !strings.Contains(hdr.Name, "/text/") {
			continue
		}

		if filepath.Ext(hdr.Name) != ".txt" {
			continue
		}

		fileBaseName := filepath.Base(hdr.Name)
		licenseName := strings.TrimSuffix(fileBaseName, ".txt")

		if strings.HasPrefix(fileBaseName, "README") {
			continue
		}

		if strings.HasPrefix(fileBaseName, "deprecated_") {
			continue
		}
		out, err := os.Create(path.Join(destination, licenseName))
		if err != nil {
			log.Fatalf("Failed to create new file. %s", err)
		}

		defer out.Close()

		// some license files have same content, so we need to detect these files and create a convert map into a json file
		// Later we use this convert map to avoid adding same license content with different license name
		h := md5.New()
		// calculate md5 and write file in the same time
		r := io.TeeReader(tr, h)
		if _, err := io.Copy(out, r); err != nil {
			log.Fatalf("Failed to write new file. %s", err)
		} else {
			fmt.Printf("Written %s\n", out.Name())

			md5 := hex.EncodeToString(h.Sum(nil))
			aliasesFiles[md5] = append(aliasesFiles[md5], licenseName)
		}
	}

	// generate convert license name map
	licenseAliases := make(map[string]string)
	for _, fileNames := range aliasesFiles {
		if len(fileNames) > 1 {
			licenseName := license.GetLicenseNameFromAliases(fileNames)
			if licenseName == "" {
				// license name should not be empty as expected
				// if it is empty, we need to rewrite the logic of GetLicenseNameFromAliases
				log.Fatalf("GetLicenseNameFromAliases: license name is empty")
			}
			for _, fileName := range fileNames {
				licenseAliases[fileName] = licenseName
			}
		}
	}
	// save convert license name map to file
	b, err := json.Marshal(licenseAliases)
	if err != nil {
		log.Fatalf("Failed to create json bytes. %s", err)
	}

	licenseAliasesDestination := filepath.Join(destination, "etc", "license-aliases.json")
	if err := os.MkdirAll(filepath.Dir(licenseAliasesDestination), 0o755); err != nil {
		log.Fatalf("Failed to create directory for license aliases json file. %s", err)
	}

	f, err := os.Create(licenseAliasesDestination)
	if err != nil {
		log.Fatalf("Failed to create license aliases json file. %s", err)
	}
	defer f.Close()

	if _, err = f.Write(b); err != nil {
		log.Fatalf("Failed to write license aliases json file. %s", err)
	}

	fmt.Println("Done")
}
