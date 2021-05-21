// Copyright (c) 2020 Klaus Post, released under MIT License. See LICENSE file.

// +build arm64
// +build !linux
// +build !darwin

package cpuid

func detectOS(c *CPUInfo) bool {
	return false
}
