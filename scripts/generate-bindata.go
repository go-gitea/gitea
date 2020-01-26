// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/shurcooL/vfsgen"
)

func needsUpdate(dir string, filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return true
	}

	lastModifiedTime := info.ModTime()

	newer := false

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if newer || info.ModTime().After(lastModifiedTime) {
			newer = true
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return true
	}

	return newer
}

func main() {
	if len(os.Args) < 4 {
		log.Fatal("Insufficient number of arguments. Need: directory packageName filename")
	}

	dir, packageName, filename := os.Args[1], os.Args[2], os.Args[3]

	if !needsUpdate(dir, filename) {
		fmt.Printf("bindata for %s up-to-date refusing to generate\n", packageName)
		return
	}

	fmt.Printf("generating bindata for %s\n", packageName)
	var fsTemplates http.FileSystem = http.Dir(dir)
	err := vfsgen.Generate(fsTemplates, vfsgen.Options{
		PackageName:  packageName,
		BuildTags:    "bindata",
		VariableName: "Assets",
		Filename:     filename,
	})
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
