// Copyright 2013, 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package errors

import (
	"runtime"
	"strings"
)

// prefixSize is used internally to trim the user specific path from the
// front of the returned filenames from the runtime call stack.
var prefixSize int

// goPath is the deduced path based on the location of this file as compiled.
var goPath string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if file == "?" {
		return
	}
	if ok {
		// We know that the end of the file should be:
		// github.com/juju/errors/path.go
		size := len(file)
		suffix := len("github.com/juju/errors/path.go")
		goPath = file[:size-suffix]
		prefixSize = len(goPath)
	}
}

func trimGoPath(filename string) string {
	if strings.HasPrefix(filename, goPath) {
		return filename[prefixSize:]
	}
	return filename
}
