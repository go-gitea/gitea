// Copyright (c) 2021 Klaus Post, released under MIT License. See LICENSE file.

//+build !nounsafe

package cpuid

import _ "unsafe" // needed for go:linkname

//go:linkname hwcap internal/cpu.HWCap
var hwcap uint
