// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !gofuzz

// This file exists to suppress fuzzing details from release builds.

package bzip2

type fuzzReader struct{}

func (*fuzzReader) updateChecksum(int64, uint32) {}
