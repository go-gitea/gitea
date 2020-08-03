// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import "strings"

// These flags define which text to prefix to each log entry generated
// by the Logger. Bits are or'ed together to control what's printed.
// There is no control over the order they appear (the order listed
// here) or the format they present (as described in the comments).
// The prefix is followed by a colon only if more than time is stated
// is specified. For example, flags Ldate | Ltime
// produce, 2009/01/23 01:23:23 message.
// The standard is:
// 2009/01/23 01:23:23 ...a/logger/c/d.go:23:runtime.Caller() [I]: message
const (
	Ldate          = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                      // the time in the local time zone: 01:23:23
	Lmicroseconds              // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                  // full file name and line number: /a/logger/c/d.go:23
	Lshortfile                 // final file name element and line number: d.go:23. overrides Llongfile
	Lfuncname                  // function name of the caller: runtime.Caller()
	Lshortfuncname             // last part of the function name
	LUTC                       // if Ldate or Ltime is set, use UTC rather than the local time zone
	Llevelinitial              // Initial character of the provided level in brackets eg. [I] for info
	Llevel                     // Provided level in brackets [INFO]

	// Last 20 characters of the filename
	Lmedfile = Lshortfile | Llongfile

	// LstdFlags is the initial value for the standard logger
	LstdFlags = Ldate | Ltime | Lmedfile | Lshortfuncname | Llevelinitial
)

var flagFromString = map[string]int{
	"none":          0,
	"date":          Ldate,
	"time":          Ltime,
	"microseconds":  Lmicroseconds,
	"longfile":      Llongfile,
	"shortfile":     Lshortfile,
	"funcname":      Lfuncname,
	"shortfuncname": Lshortfuncname,
	"utc":           LUTC,
	"levelinitial":  Llevelinitial,
	"level":         Llevel,
	"medfile":       Lmedfile,
	"stdflags":      LstdFlags,
}

// FlagsFromString takes a comma separated list of flags and returns
// the flags for this string
func FlagsFromString(from string) int {
	flags := 0
	for _, flag := range strings.Split(strings.ToLower(from), ",") {
		f, ok := flagFromString[strings.TrimSpace(flag)]
		if ok {
			flags |= f
		}
	}
	if flags == 0 {
		return -1
	}
	return flags
}
