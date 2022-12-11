// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// gobuild: external_renderer

package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Print(os.Args[1])
	} else {
		_, err := io.Copy(os.Stdout, os.Stdin)
		if err != nil {
			fmt.Println(err)
		}
	}
}
