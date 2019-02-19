// Copyright 2013 Martini Authors
// Copyright 2014 The Macaron Authors
// Copyright 2019 The Gitea Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package context

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"runtime"

	macaron "gopkg.in/macaron.v1"
)

// Recovery returns a middleware that recovers from any panics and writes a 500 and a log if so.
// Although similar to macaron.Recovery() the main difference is that this error will be created
// with the gitea 500 page.
func Recovery() macaron.Handler {
	return func(ctx *Context) {
		defer func() {
			if err := recover(); err != nil {
				combinedErr := fmt.Errorf("%s\n%s", err, string(stack(3)))
				ctx.ServerError("PANIC:", combinedErr)
			}
		}()

		ctx.Next()
	}
}

var (
	unknown = []byte("???")
)

// Although we could just use debug.Stack(), this routine will return the source code
// skip the provided number of frames - i.e. allowing us to ignore this function call
// and the preceding function call.
// If the problem is a lack of memory of course all this is not going to work...
func stack(skip int) []byte {
	buf := new(bytes.Buffer)

	// Store the last file we opened as its probable that the preceding stack frame
	// will be in the same file
	var lines [][]byte
	var lastFilename string
	for i := skip; ; i++ { // Skip over frames
		programCounter, filename, lineNumber, ok := runtime.Caller(i)
		// If we can't retrieve the information break - basically we're into go internals at this point.
		if !ok {
			break
		}

		// Print equivalent of debug.Stack()
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", filename, lineNumber, programCounter)
		// Now try to print the offending line
		if filename != lastFilename {
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				// can't read this sourcefile
				// likely we don't have the sourcecode available
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFilename = filename
		}
		fmt.Fprintf(buf, "\t%s: %s\n", functionName(programCounter), source(lines, lineNumber))
	}
	return buf.Bytes()
}

// functionName converts the provided programCounter into a function name
func functionName(programCounter uintptr) []byte {
	function := runtime.FuncForPC(programCounter)
	if function == nil {
		return unknown
	}
	name := []byte(function.Name())

	// Because we provide the filename we can drop the preceding package name.
	if lastslash := bytes.LastIndex(name, []byte("/")); lastslash >= 0 {
		name = name[lastslash+1:]
	}
	// And the current package name.
	if period := bytes.Index(name, []byte(".")); period >= 0 {
		name = name[period+1:]
	}
	// And we should just replace the interpunct with a dot
	name = bytes.Replace(name, []byte("·"), []byte("."), -1)
	return name
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return unknown
	}
	return bytes.TrimSpace(lines[n])
}
