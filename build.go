// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build vendor

package main

// Libraries that are included to vendor utilities used during Makefile build.
// These libraries will not be included in a normal compilation.

import (
	// for vet
	_ "code.gitea.io/gitea-vet"
)
