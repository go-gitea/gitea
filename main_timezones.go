// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package main

// Golang has the ability to load OS's timezone data from most UNIX systems (https://github.com/golang/go/blob/master/src/time/zoneinfo_unix.go)
// Even if the timezone data is missing, users could install the related packages to get it.
// But on Windows, although `zoneinfo_windows.go` tries to load the timezone data from Windows registry,
// some users still suffer from the issue that the timezone data is missing: https://github.com/go-gitea/gitea/issues/33235
// So we import the tzdata package to make sure the timezone data is included in the binary.
//
// For non-Windows package builders, they could still use the "TAGS=timetzdata" to include the tzdata package in the binary.
// If we decided to add the tzdata for other platforms, modify the "go:build" directive above.
import _ "time/tzdata"
