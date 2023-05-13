// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type Event struct {
	Time time.Time

	GoroutinePid string
	Caller       string
	Filename     string
	Line         int

	Level Level

	Msg           string
	MsgFormat     string
	MsgFrozenArgs []any // it may contains *ColorValue

	Stacktrace string
}

type EventFormatter func(mode *WriterMode, event *Event, reuse []byte) []byte

type frozenMsgArg struct {
	m *frozenMsgFormatter
	v any
	s string

	processed bool
}

func (a *frozenMsgArg) Format(s fmt.State, c rune) {
	a.s = fmt.Sprintf(fmt.FormatString(s, c), a.v)
	_, _ = s.Write([]byte(a.s))
	a.processed = true
}

type frozenMsgFormatter struct {
	format string
	args   []any
}

func (m *frozenMsgFormatter) addArgs(args ...any) {
	for _, v := range args {
		switch v := v.(type) {
		case fmt.Stringer, fmt.GoStringer, LogStringer:
			m.args = append(m.args, &frozenMsgArg{m: m, v: v})
		default:
			m.args = append(m.args, v)
		}
	}
}

func (m *frozenMsgFormatter) doFormat() string {
	res := fmt.Sprintf(m.format, m.args...)
	for i := range m.args {
		if arg, ok := m.args[i].(*frozenMsgArg); ok {
			if arg.processed {
				m.args[i] = arg.s
			} else {
				switch v := arg.v.(type) {
				case LogStringer:
					m.args[i] = v.LogString()
				case fmt.GoStringer: // GoString() is for "%#v" only, but it's also fine to freeze the argument by it
					m.args[i] = v.GoString()
				case fmt.Stringer:
					m.args[i] = v.String()
				default:
					m.args[i] = v
				}
			}
		}
	}
	return res
}

func frozenMsgFormat(format string, args ...any) (msg string, frozenArgs []any) {
	m := frozenMsgFormatter{format: format}
	m.addArgs(args...)
	msg = m.doFormat()
	return msg, m.args
}

// Copy of cheap integer to fixed-width decimal to ascii from logger.
func itoa(buf []byte, i, wid int) []byte {
	var s [20]byte
	bp := len(s) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		s[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	s[bp] = byte('0' + i)
	return append(buf, s[bp:]...)
}

// EventFormatTextMessage makes the log message for a writer with its mode. This function is a copy of the original package
func EventFormatTextMessage(mode *WriterMode, event *Event, buf []byte) []byte {
	buf = append(buf, mode.Prefix...)
	t := event.Time
	if mode.Flags&(Ldate|Ltime|Lmicroseconds) != 0 {
		if mode.Colorize {
			buf = append(buf, fgCyanBytes...)
		}
		if mode.Flags&LUTC != 0 {
			t = t.UTC()
		}
		if mode.Flags&Ldate != 0 {
			year, month, day := t.Date()
			buf = itoa(buf, year, 4)
			buf = append(buf, '/')
			buf = itoa(buf, int(month), 2)
			buf = append(buf, '/')
			buf = itoa(buf, day, 2)
			buf = append(buf, ' ')
		}
		if mode.Flags&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			buf = itoa(buf, hour, 2)
			buf = append(buf, ':')
			buf = itoa(buf, min, 2)
			buf = append(buf, ':')
			buf = itoa(buf, sec, 2)
			if mode.Flags&Lmicroseconds != 0 {
				buf = append(buf, '.')
				buf = itoa(buf, t.Nanosecond()/1e3, 6)
			}
			buf = append(buf, ' ')
		}
		if mode.Colorize {
			buf = append(buf, resetBytes...)
		}

	}
	if mode.Flags&(Lshortfile|Llongfile) != 0 {
		if mode.Colorize {
			buf = append(buf, fgGreenBytes...)
		}
		file := event.Filename
		if mode.Flags&Lmedfile == Lmedfile {
			startIndex := len(file) - 20
			if startIndex > 0 {
				file = "..." + file[startIndex:]
			}
		} else if mode.Flags&Lshortfile != 0 {
			startIndex := strings.LastIndexByte(file, '/')
			if startIndex > 0 && startIndex < len(file) {
				file = file[startIndex+1:]
			}
		}
		buf = append(buf, file...)
		buf = append(buf, ':')
		buf = itoa(buf, event.Line, -1)
		if mode.Flags&(Lfuncname|Lshortfuncname) != 0 {
			buf = append(buf, ':')
		} else {
			if mode.Colorize {
				buf = append(buf, resetBytes...)
			}
			buf = append(buf, ' ')
		}
	}
	if mode.Flags&(Lfuncname|Lshortfuncname) != 0 {
		if mode.Colorize {
			buf = append(buf, fgGreenBytes...)
		}
		funcname := event.Caller
		if mode.Flags&Lshortfuncname != 0 {
			lastIndex := strings.LastIndexByte(funcname, '.')
			if lastIndex > 0 && len(funcname) > lastIndex+1 {
				funcname = funcname[lastIndex+1:]
			}
		}
		buf = append(buf, funcname...)
		if mode.Colorize {
			buf = append(buf, resetBytes...)
		}
		buf = append(buf, ' ')
	}

	if mode.Flags&(Llevel|Llevelinitial) != 0 {
		level := strings.ToUpper(event.Level.String())
		if mode.Colorize {
			buf = append(buf, ColorBytes(levelToColor[event.Level]...)...)
		}
		buf = append(buf, '[')
		if mode.Flags&Llevelinitial != 0 {
			buf = append(buf, level[0])
		} else {
			buf = append(buf, level...)
		}
		buf = append(buf, ']')
		if mode.Colorize {
			buf = append(buf, resetBytes...)
		}
		buf = append(buf, ' ')
	}

	msg := []byte(event.Msg)
	if mode.Colorize {
		hasColorValue := false
		for _, v := range event.MsgFrozenArgs {
			if _, hasColorValue = v.(*ColoredValue); hasColorValue {
				break
			}
		}
		if hasColorValue {
			msg = []byte(fmt.Sprintf(event.MsgFormat, event.MsgFrozenArgs...))
		}
	}
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	if mode.Flags&Lgopid == Lgopid {
		if event.GoroutinePid != "" {
			buf = append(buf, '[')
			if mode.Colorize {
				buf = append(buf, ColorBytes(FgHiYellow)...)
			}
			buf = append(buf, event.GoroutinePid...)
			if mode.Colorize {
				buf = append(buf, resetBytes...)
			}
			buf = append(buf, ']', ' ')
		}
	}
	buf = append(buf, msg...)

	if event.Stacktrace != "" && mode.StacktraceLevel <= event.Level {
		lines := bytes.Split([]byte(event.Stacktrace), []byte("\n"))
		if len(lines) > 1 {
			for _, line := range lines {
				buf = append(buf, "\n\t"...)
				buf = append(buf, line...)
			}
		}
		buf = append(buf, '\n')
	}
	buf = append(buf, '\n')
	return buf
}
