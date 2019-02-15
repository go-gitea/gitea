// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"io"
	"regexp"
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
// 2009/01/23 01:23:23 /a/b/c/d.go:23:runtime.Caller() [I]: message
const (
	Ldate         = 1 << iota                                             // the date in the local time zone: 2009/01/23
	Ltime                                                                 // the time in the local time zone: 01:23:23
	Lmicroseconds                                                         // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                                                             // full file name and line number: /a/b/c/d.go:23
	Lshortfile                                                            // final file name element and line number: d.go:23. overrides Llongfile
	Lfuncname                                                             // function name of the caller: runtime.Caller()
	LUTC                                                                  // if Ldate or Ltime is set, use UTC rather than the local time zone
	Llevelinitial                                                         // Initial character of the provided level in brackets eg. [I] for info
	Llevel                                                                // Provided level in brackets [INFO]
	LstdFlags     = Ldate | Ltime | Llongfile | Lfuncname | Llevelinitial // initial values for the standard logger
)

// BaseLogger represent a basic logger for Gitea
type BaseLogger struct {
	out io.WriteCloser
	mu  sync.Mutex

	Level Level `json:"level"`

	Flags      int    `json:"flags"`
	Prefix     string `json:"prefix"`
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
	}
	if b.Flags&(Lshortfile|Llongfile) != 0 {
		file := event.filename
		if b.Flags&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, event.line, -1)
		if b.Flags&Lfuncname != 0 {
			*buf = append(*buf, ':')
		} else {
			*buf = append(*buf, ' ')
		}
	}
	if b.Flags&Lfuncname != 0 {
		*buf = append(*buf, event.caller...)
		*buf = append(*buf, ' ')
	}
	if b.Flags&(Llevel|Llevelinitial) != 0 {
		level := event.level.String()
		*buf = append(*buf, '[')
		if b.Flags&Llevelinitial != 0 {
			*buf = append(*buf, level[0])
		} else {
			*buf = append(*buf, level...)
		}
		*buf = append(*buf, "] "...)
	}
	*buf = append(*buf, event.msg...)
	if len(event.msg) == 0 || event.msg[len(event.msg)-1] != '\n' {
		*buf = append(*buf, '\n')
	}
}

// LogEvent logs the event to the internal writer
func (b *BaseLogger) LogEvent(event *Event) error {
	if b.Level > event.level {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	var buf []byte
	b.createMsg(&buf, event)
	if b.regexp != nil && !b.regexp.Match(buf) {
		return nil
	}
	_, err := b.out.Write(buf)
	return err
}

// Close the base logger
func (b *BaseLogger) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.out.Close()
}
