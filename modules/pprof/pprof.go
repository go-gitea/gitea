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
func DumpMemProfileForUsername(pprofDataPath, username string) error {
	f, err := ioutil.TempFile(pprofDataPath, fmt.Sprintf("memprofile_%s_", username))
	if err != nil {
		return err
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	return pprof.WriteHeapProfile(f)
}

// DumpCPUProfileForUsername dumps a CPU profile at pprofDataPath as cpuprofile_<username>_<temporary id>
//  it returns the stop function which stops, writes and closes the CPU profile file
func DumpCPUProfileForUsername(pprofDataPath, username string) (func(), error) {
	f, err := ioutil.TempFile(pprofDataPath, fmt.Sprintf("cpuprofile_%s_", username))
	if err != nil {
		return nil, err
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Fatal("StartCPUProfile: %v", err)
	}
	return func() {
		pprof.StopCPUProfile()
		err = f.Close()
		if err != nil {
			log.Fatal("StopCPUProfile Close: %v", err)
		}
	}, nil
}
