// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/json"
)

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
	Ldate          uint32 = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                             // the time in the local time zone: 01:23:23
	Lmicroseconds                     // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                         // full file name and line number: /a/logger/c/d.go:23
	Lshortfile                        // final file name element and line number: d.go:23. overrides Llongfile
	Lfuncname                         // function name of the caller: runtime.Caller()
	Lshortfuncname                    // last part of the function name
	LUTC                              // if Ldate or Ltime is set, use UTC rather than the local time zone
	Llevelinitial                     // Initial character of the provided level in brackets, eg. [I] for info
	Llevel                            // Provided level in brackets [INFO]
	Lgopid                            // the Goroutine-PID of the context

	Lmedfile  = Lshortfile | Llongfile                                    // last 20 characters of the filename
	LstdFlags = Ldate | Ltime | Lmedfile | Lshortfuncname | Llevelinitial // default
)

const Ldefault = LstdFlags

type Flags struct {
	defined bool
	flags   uint32
}

var flagFromString = map[string]uint32{
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
	"gopid":         Lgopid,

	"medfile":  Lmedfile,
	"stdflags": LstdFlags,
}

var flagComboToString = []struct {
	flag uint32
	name string
}{
	// name with more bits comes first
	{LstdFlags, "stdflags"},
	{Lmedfile, "medfile"},

	{Ldate, "date"},
	{Ltime, "time"},
	{Lmicroseconds, "microseconds"},
	{Llongfile, "longfile"},
	{Lshortfile, "shortfile"},
	{Lfuncname, "funcname"},
	{Lshortfuncname, "shortfuncname"},
	{LUTC, "utc"},
	{Llevelinitial, "levelinitial"},
	{Llevel, "level"},
	{Lgopid, "gopid"},
}

func (f Flags) Bits() uint32 {
	if !f.defined {
		return Ldefault
	}
	return f.flags
}

func (f Flags) String() string {
	flags := f.Bits()
	var flagNames []string
	for _, it := range flagComboToString {
		if flags&it.flag == it.flag {
			flags &^= it.flag
			flagNames = append(flagNames, it.name)
		}
	}
	if len(flagNames) == 0 {
		return "none"
	}
	sort.Strings(flagNames)
	return strings.Join(flagNames, ",")
}

func (f *Flags) UnmarshalJSON(bytes []byte) error {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	*f = FlagsFromString(s)
	return nil
}

func (f Flags) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

func FlagsFromString(from string, def ...uint32) Flags {
	from = strings.TrimSpace(from)
	if from == "" && len(def) > 0 {
		return Flags{defined: true, flags: def[0]}
	}
	flags := uint32(0)
	for _, flag := range strings.Split(strings.ToLower(from), ",") {
		flags |= flagFromString[strings.TrimSpace(flag)]
	}
	return Flags{defined: true, flags: flags}
}

func FlagsFromBits(flags uint32) Flags {
	return Flags{defined: true, flags: flags}
}
