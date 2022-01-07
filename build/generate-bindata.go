// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/shurcooL/vfsgen"
)

func needsUpdate(dir string, filename string) (bool, []byte) {
	needRegen := false
	_, err := os.Stat(filename)
	if err != nil {
		needRegen = true
	}

	oldHash, err := os.ReadFile(filename + ".hash")
	if err != nil {
		oldHash = []byte{}
	}

	hasher := sha1.New()

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		_, _ = hasher.Write([]byte(info.Name()))
		_, _ = hasher.Write([]byte(info.ModTime().String()))
		_, _ = hasher.Write([]byte(strconv.FormatInt(info.Size(), 16)))
		return nil
	})
	if err != nil {
		return true, oldHash
	}

	newHash := hasher.Sum([]byte{})

	if bytes.Compare(oldHash, newHash) != 0 {

		return true, newHash
	}

	return needRegen, newHash
}

func main() {
	if len(os.Args) < 4 {
		log.Fatal("Insufficient number of arguments. Need: directory packageName filename")
	}

	dir, packageName, filename := os.Args[1], os.Args[2], os.Args[3]
	var useGlobalModTime bool
	if len(os.Args) == 5 {
		useGlobalModTime, _ = strconv.ParseBool(os.Args[4])
	}

	update, newHash := needsUpdate(dir, filename)

	if !update {
		fmt.Printf("bindata for %s already up-to-date\n", packageName)
		return
	}

	fmt.Printf("generating bindata for %s\n", packageName)
	var fsTemplates http.FileSystem = http.Dir(dir)
	err := vfsgen.Generate(fsTemplates, vfsgen.Options{
		PackageName:      packageName,
		BuildTags:        "bindata",
		VariableName:     "Assets",
		Filename:         filename,
		UseGlobalModTime: useGlobalModTime,
	})
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	_ = os.WriteFile(filename+".hash", newHash, 0666)
}
