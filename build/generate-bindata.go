// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/assetfs"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: ./generate-bindata {local-directory} {bindata-filename}")
		os.Exit(1)
	}

	dir, filename := os.Args[1], os.Args[2]
	fmt.Printf("generating bindata for %s to %s\n", dir, filename)
	if err := assetfs.GenerateEmbedBindata(dir, filename); err != nil {
		fmt.Printf("failed: %s\n", err.Error())
		os.Exit(1)
	}
}
