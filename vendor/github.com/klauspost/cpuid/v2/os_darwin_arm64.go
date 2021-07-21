// Copyright (c) 2020 Klaus Post, released under MIT License. See LICENSE file.

package cpuid

import "runtime"

func detectOS(c *CPUInfo) bool {
	// There are no hw.optional sysctl values for the below features on Mac OS 11.0
	// to detect their supported state dynamically. Assume the CPU features that
	// Apple Silicon M1 supports to be available as a minimal set of features
	// to all Go programs running on darwin/arm64.
	// TODO: Add more if we know them.
	c.featureSet.setIf(runtime.GOOS != "ios", AESARM, PMULL, SHA1, SHA2)
	c.PhysicalCores = runtime.NumCPU()
	// For now assuming 1 thread per core...
	c.ThreadsPerCore = 1
	c.LogicalCores = c.PhysicalCores
	return true
}
