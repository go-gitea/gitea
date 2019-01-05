// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pprof

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"runtime/pprof"

	"code.gitea.io/gitea/modules/log"
)

// DumpMemProfileForUsername dumps a memory profile at pprofDataPath as memprofile_<username>_<temporary id>
func DumpMemProfileForUsername(pprofDataPath, username string) {
	f, err := ioutil.TempFile(pprofDataPath, fmt.Sprintf("memprofile_%s_", username))
	if err != nil {
		log.GitLogger.Fatal(4, "Could not create memory profile: %v", err)
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.GitLogger.Fatal(4, "Could not write memory profile: %v", err)
	}
}

// DumpCPUProfileForUsername dumps a CPU profile at pprofDataPath as cpuprofile_<username>_<temporary id>
//  it returns the stop function which stops, writes and closes the CPU profile file
func DumpCPUProfileForUsername(pprofDataPath, username string) func() {
	f, err := ioutil.TempFile(pprofDataPath, fmt.Sprintf("cpuprofile_%s_", username))
	if err != nil {
		log.GitLogger.Fatal(4, "Could not create cpu profile: %v", err)
	}

	pprof.StartCPUProfile(f)
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}
