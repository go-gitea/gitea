// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

var (
	funcInfoMap     = map[uintptr]*FuncInfo{}
	funcInfoNameMap = map[string]*FuncInfo{}
	funcInfoMapMu   sync.RWMutex
)

// FuncInfo contains information about the function to be logged by the router log
type FuncInfo struct {
	file      string
	shortFile string
	line      int
	name      string
	shortName string
}

// String returns a string form of the FuncInfo for logging
func (info *FuncInfo) String() string {
	if info == nil {
		return "unknown-handler"
	}
	return fmt.Sprintf("%s:%d(%s)", info.shortFile, info.line, info.shortName)
}

// GetFuncInfo returns the FuncInfo for a provided function and friendlyname
func GetFuncInfo(fn any, friendlyName ...string) *FuncInfo {
	// ptr represents the memory position of the function passed in as v.
	// This will be used as program counter in FuncForPC below
	ptr := reflect.ValueOf(fn).Pointer()

	// if we have been provided with a friendlyName look for the named funcs
	if len(friendlyName) == 1 {
		name := friendlyName[0]
		funcInfoMapMu.RLock()
		info, ok := funcInfoNameMap[name]
		funcInfoMapMu.RUnlock()
		if ok {
			return info
		}
	}

	// Otherwise attempt to get pre-cached information for this function pointer
	funcInfoMapMu.RLock()
	info, ok := funcInfoMap[ptr]
	funcInfoMapMu.RUnlock()

	if ok {
		if len(friendlyName) == 1 {
			name := friendlyName[0]
			info = copyFuncInfo(info)
			info.shortName = name

			funcInfoNameMap[name] = info
			funcInfoMapMu.Lock()
			funcInfoNameMap[name] = info
			funcInfoMapMu.Unlock()
		}
		return info
	}

	// This is likely the first time we have seen this function
	//
	// Get the runtime.func for this function (if we can)
	f := runtime.FuncForPC(ptr)
	if f != nil {
		info = convertToFuncInfo(f)

		// cache this info globally
		funcInfoMapMu.Lock()
		funcInfoMap[ptr] = info

		// if we have been provided with a friendlyName override the short name we've generated
		if len(friendlyName) == 1 {
			name := friendlyName[0]
			info = copyFuncInfo(info)
			info.shortName = name
			funcInfoNameMap[name] = info
		}
		funcInfoMapMu.Unlock()
	}
	return info
}

// convertToFuncInfo take a runtime.Func and convert it to a logFuncInfo, fill in shorten filename, etc
func convertToFuncInfo(f *runtime.Func) *FuncInfo {
	file, line := f.FileLine(f.Entry())

	info := &FuncInfo{
		file: strings.ReplaceAll(file, "\\", "/"),
		line: line,
		name: f.Name(),
	}

	// only keep last 2 names in path, fall back to funcName if not
	info.shortFile = shortenFilename(info.file, info.name)

	// remove package prefix. eg: "xxx.com/pkg1/pkg2.foo" => "pkg2.foo"
	pos := strings.LastIndexByte(info.name, '/')
	if pos >= 0 {
		info.shortName = info.name[pos+1:]
	} else {
		info.shortName = info.name
	}

	// remove ".func[0-9]*" suffix for anonymous func
	info.shortName = trimAnonymousFunctionSuffix(info.shortName)

	return info
}

func copyFuncInfo(l *FuncInfo) *FuncInfo {
	return &FuncInfo{
		file:      l.file,
		shortFile: l.shortFile,
		line:      l.line,
		name:      l.name,
		shortName: l.shortName,
	}
}

// shortenFilename generates a short source code filename from a full package path, eg: "code.gitea.io/routers/common/logger_context.go" => "common/logger_context.go"
func shortenFilename(filename, fallback string) string {
	if filename == "" {
		return fallback
	}
	if lastIndex := strings.LastIndexByte(filename, '/'); lastIndex >= 0 {
		if secondLastIndex := strings.LastIndexByte(filename[:lastIndex], '/'); secondLastIndex >= 0 {
			return filename[secondLastIndex+1:]
		}
	}
	return filename
}

// trimAnonymousFunctionSuffix trims ".func[0-9]*" from the end of anonymous function names, we only want to see the main function names in logs
func trimAnonymousFunctionSuffix(name string) string {
	// if the name is an anonymous name, it should be like "{main-function}.func1", so the length can not be less than 7
	if len(name) < 7 {
		return name
	}

	funcSuffixIndex := strings.LastIndex(name, ".func")
	if funcSuffixIndex < 0 {
		return name
	}

	hasFuncSuffix := true

	// len(".func") = 5
	for i := funcSuffixIndex + 5; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			hasFuncSuffix = false
			break
		}
	}

	if hasFuncSuffix {
		return name[:funcSuffixIndex]
	}
	return name
}
