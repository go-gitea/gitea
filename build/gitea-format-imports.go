// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

package main

import (
	"log"
	"os"

	"code.gitea.io/gitea/build/codeformat"
)

func main() {
	if len(os.Args) <= 1 {
		log.Fatalf("Usage: gitea-format-imports [files...]")
	}

	for _, file := range os.Args[1:] {
		if err := codeformat.FormatGoImports(file); err != nil {
			log.Fatalf("can not format file %s, err=%v", file, err)
		}
	}
}
