// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

// These flags define which text to prefix to each log entry generated
// by the Logger. Bits are or'ed together to control what's printed.
// There is no control over the order they appear (the order listed
// here) or the format they present (as described in the comments).
// The prefix is followed by a colon only if more than time is stated
// is specified. For example, flags Ldate | Ltime
// produce, 2009/01/23 01:23:23 message.
// The standard is:
// 2009/01/23 01:23:23 ...a/b/c/d.go:23:runtime.Caller() [I]: message
const (
	Ldate          = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                      // the time in the local time zone: 01:23:23
	Lmicroseconds              // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                  // full file name and line number: /a/b/c/d.go:23
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

// BaseLogger represent a basic logger for Gitea
type BaseLogger struct {
	out io.WriteCloser
	mu  sync.Mutex

	Level      Level  `json:"level"`
	Flags      int    `json:"flags"`
	Prefix     string `json:"prefix"`
	Colorize   bool   `json:"colorize"`
	Expression string `json:"expression"`
	regexp     *regexp.Regexp
}

func (b *BaseLogger) createLogger(out io.WriteCloser, level ...Level) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.out = out
	switch b.Flags {
	case 0:
		b.Flags = LstdFlags
	case -1:
		b.Flags = 0
	}
	if len(level) > 0 {
		b.Level = level[0]
	}
	b.createExpression()
}

func (b *BaseLogger) createExpression() {
	if len(b.Expression) > 0 {
		var err error
		b.regexp, err = regexp.Compile(b.Expression)
		if err != nil {
			b.regexp = nil
		}
	}
}

// GetLevel returns the logging level for this logger
func (b *BaseLogger) GetLevel() Level {
	return b.Level
}

// Copy of cheap integer to fixed-width decimal to ascii from logger.
func itoa(buf *[]byte, i int, wid int) {
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func (b *BaseLogger) createMsg(buf *[]byte, event *Event) {
	*buf = append(*buf, b.Prefix...)
	t := event.time
	if b.Flags&(Ldate|Ltime|Lmicroseconds) != 0 {
		if b.Colorize {
			*buf = append(*buf, fgCyanString...)
		}
		if b.Flags&LUTC != 0 {
			t = t.UTC()
		}
		if b.Flags&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if b.Flags&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if b.Flags&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
		if b.Colorize {
			*buf = append(*buf, resetString...)
		}

	}
	if b.Flags&(Lshortfile|Llongfile) != 0 {
		if b.Colorize {
			*buf = append(*buf, fgGreenString...)
		}
		file := event.filename
		if b.Flags&Lmedfile == Lmedfile {
			startIndex := len(file) - 20
			if startIndex > 0 {
				file = "..." + file[startIndex:]
			}
		} else if b.Flags&Lshortfile != 0 {
			startIndex := strings.LastIndexByte(file, '/')
			if startIndex > 0 && startIndex < len(file) {
				file = file[startIndex+1:]
			}
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, event.line, -1)
		if b.Flags&(Lfuncname|Lshortfuncname) != 0 {
			*buf = append(*buf, ':')
		} else {
			if b.Colorize {
				*buf = append(*buf, resetString...)
			}
			*buf = append(*buf, ' ')
		}
	}
	if b.Flags&(Lfuncname|Lshortfuncname) != 0 {
		if b.Colorize {
			*buf = append(*buf, fgGreenString...)
		}
		funcname := event.caller
		if b.Flags&Lshortfuncname != 0 {
			lastIndex := strings.LastIndexByte(funcname, '.')
			if lastIndex > 0 && len(funcname) > lastIndex+1 {
				funcname = funcname[lastIndex+1:]
			}
		}
		*buf = append(*buf, funcname...)
		if b.Colorize {
			*buf = append(*buf, resetString...)
		}
		*buf = append(*buf, ' ')

	}
	if b.Flags&(Llevel|Llevelinitial) != 0 {
		level := strings.ToUpper(event.level.String())
		if b.Colorize {
			*buf = append(*buf, levelToColor[event.level]...)
		}
		*buf = append(*buf, '[')
		if b.Flags&Llevelinitial != 0 {
			*buf = append(*buf, level[0])
		} else {
			*buf = append(*buf, level...)
		}
		*buf = append(*buf, ']')
		if b.Colorize {
			*buf = append(*buf, resetString...)
		}
		*buf = append(*buf, ' ')
	}
	// Now we need to prevent log spoofing:
	if len(event.msg) > 0 && event.msg[len(event.msg)-1] == '\n' {
		event.msg = event.msg[:len(event.msg)-1]
	}
	lines := bytes.Split([]byte(event.msg), []byte("\n"))
	*buf = append(*buf, lines[0]...)
	if len(lines) > 1 {
		for _, line := range lines[1:] {
			*buf = append(*buf, "\n        "...)
			*buf = append(*buf, line...)
		}
	}
	*buf = append(*buf, '\n')
}

// LogEvent logs the event to the internal writer
func (b *BaseLogger) LogEvent(event *Event) error {
	if b.Level > event.level {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.Match(event) {
		return nil
	}
	var buf []byte
	b.createMsg(&buf, event)
	_, err := b.out.Write(buf)
	return err
}

// Match checks if the given event matches the logger's regexp expression
func (b *BaseLogger) Match(event *Event) bool {
	if b.regexp == nil {
		return true
	}
	if b.regexp.Match([]byte(fmt.Sprintf("%s:%d:%s", event.filename, event.line, event.caller))) {
		return true
	}
	if b.regexp.Match([]byte(event.msg)) {
		return true
	}
	return false
}

// Close the base logger
func (b *BaseLogger) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.out != nil {
		b.out.Close()
	}
}

// GetName returns empty for these provider loggers
func (b *BaseLogger) GetName() string {
	return ""
}
