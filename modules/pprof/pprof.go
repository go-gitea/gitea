// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pprof

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	"gitea.dev/modules/log"
)

func dumpMemProfileForUsername(pprofDataPath, subName string) error {
	f, err := os.CreateTemp(pprofDataPath, fmt.Sprintf("pprof_mem_%s_", filepath.Clean(subName)))
	if err != nil {
		return err
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	return pprof.WriteHeapProfile(f)
}

func DumpPprofForUsername(pprofDataPath, subName string) (func(), error) {
	if err := os.MkdirAll(pprofDataPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf(`os.MkdirAll(pprofDataPath) failed: %v`, err)
	}

	f, err := os.CreateTemp(pprofDataPath, fmt.Sprintf("pprof_cpu_%s_", filepath.Clean(subName)))
	if err != nil {
		return nil, err
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("StartCPUProfile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		err = f.Close()
		if err != nil {
			log.Error("StopCPUProfile Close: %v", err)
		}
		err = dumpMemProfileForUsername(pprofDataPath, subName)
		if err != nil {
			log.Error("DumpMemProfile: %v", err)
		}
	}, nil
}
