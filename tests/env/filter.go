// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package env

import (
	"os"
	"strings"
)

func Filter(include, exclude []string) {
	env := os.Environ()
	for _, v := range env {
		included := false
		for _, i := range include {
			if strings.HasPrefix(v, i) {
				included = true
				break
			}
		}
		if !included {
			for _, e := range exclude {
				if strings.HasPrefix(v, e) {
					parts := strings.SplitN(v, "=", 2)
					os.Unsetenv(parts[0])
					break
				}
			}
		}
	}
}
