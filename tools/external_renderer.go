// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
