// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"strings"
)

// Flags represents the logging flags for a logger
type Flags int

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
	Ldate          Flags = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                            // the time in the local time zone: 01:23:23
	Lmicroseconds                    // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                        // full file name and line number: /a/logger/c/d.go:23
	Lshortfile                       // final file name element and line number: d.go:23. overrides Llongfile
	Lfuncname                        // function name of the caller: runtime.Caller()
	Lshortfuncname                   // last part of the function name
	LUTC                             // if Ldate or Ltime is set, use UTC rather than the local time zone
	Llevelinitial                    // Initial character of the provided level in brackets eg. [I] for info
	Llevel                           // Provided level in brackets [INFO]

	// Last 20 characters of the filename
	Lmedfile = Lshortfile | Llongfile

	// LstdFlags is the initial value for the standard logger
	LstdFlags = Ldate | Ltime | Lmedfile | Lshortfuncname | Llevelinitial
)

var flagOrder = []string{
	"date",
	"time",
	"microseconds",
	"longfile",
	"shortfile",
	"funcname",
	"shortfuncname",
	"utc",
	"levelinitial",
	"level",
}

// UnmarshalJSON converts a series of bytes to a flag
func (f *Flags) UnmarshalJSON(b []byte) error {
	// OK first of all try an int
	var i int
	if err := json.Unmarshal(b, &i); err == nil {
		*f = Flags(i)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*f = FlagsFromString(s)
	return nil
}

// MarshalJSON converts Flags to JSON
func (f Flags) MarshalJSON() ([]byte, error) {
	stringFlags := []string{}
	w := f
	if w&LstdFlags == LstdFlags {
		stringFlags = append(stringFlags, "stdflags")
		w = w ^ LstdFlags
	}
	if w&Lmedfile == Lmedfile {
		stringFlags = append(stringFlags, "medfile")
		w = w ^ Lmedfile
	}
	for i, k := range flagOrder {
		v := Flags(1 << uint64(i))
		if w&v == v && v != 0 {
			stringFlags = append(stringFlags, k)
		}
	}
	if len(stringFlags) == 0 {
		stringFlags = append(stringFlags, "none")
	}
	return json.Marshal(strings.Join(stringFlags, ", "))
}

// FlagsFromString takes a comma separated list of flags and returns
// the flags for this string
func FlagsFromString(from string) Flags {
	flags := Flags(0)
	for _, flag := range strings.Split(strings.ToLower(from), ",") {
		for f, k := range flagOrder {
			if k == strings.TrimSpace(flag) {
				flags = flags | Flags(1<<uint64(f))
			}
		}
	}
	return flags
}
