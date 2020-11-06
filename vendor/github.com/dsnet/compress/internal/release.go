// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !debug,!gofuzz

package internal

// Debug indicates whether the debug build tag was set.
//
// If set, programs may choose to print with more human-readable
// debug information and also perform sanity checks that would otherwise be too
// expensive to run in a release build.
const Debug = false

// GoFuzz indicates whether the gofuzz build tag was set.
//
// If set, programs may choose to disable certain checks (like checksums) that
// would be nearly impossible for gofuzz to properly get right.
// If GoFuzz is set, it implies that Debug is set as well.
const GoFuzz = false
