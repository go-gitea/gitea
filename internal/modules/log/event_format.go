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

	MsgSimpleText string

	msgFormat string // the format and args is only valid in the caller's goroutine
	msgArgs   []any  // they are discarded before the event is passed to the writer's channel

	Stacktrace string
}

type EventFormatted struct {
	Origin *Event
	Msg    any // the message formatted by the writer's formatter, the writer knows its type
}

type EventFormatter func(mode *WriterMode, event *Event, msgFormat string, msgArgs ...any) []byte

type logStringFormatter struct {
	v LogStringer
}

var _ fmt.Formatter = logStringFormatter{}

func (l logStringFormatter) Format(f fmt.State, verb rune) {
	if f.Flag('#') && verb == 'v' {
		_, _ = fmt.Fprintf(f, "%#v", l.v)
		return
	}
	_, _ = f.Write([]byte(l.v.LogString()))
}

// Copy of cheap integer to fixed-width decimal to ascii from logger.
// TODO: legacy bugs: doesn't support negative number, overflow if wid it too large.
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

func colorSprintf(colorize bool, format string, args ...any) string {
	hasColorValue := false
	for _, v := range args {
		if _, hasColorValue = v.(*ColoredValue); hasColorValue {
			break
		}
	}
	if colorize || !hasColorValue {
		return fmt.Sprintf(format, args...)
	}

	noColors := make([]any, len(args))
	copy(noColors, args)
	for i, v := range args {
		if cv, ok := v.(*ColoredValue); ok {
			noColors[i] = cv.v
		}
	}
	return fmt.Sprintf(format, noColors...)
}

// EventFormatTextMessage makes the log message for a writer with its mode. This function is a copy of the original package
func EventFormatTextMessage(mode *WriterMode, event *Event, msgFormat string, msgArgs ...any) []byte {
	buf := make([]byte, 0, 1024)
	buf = append(buf, mode.Prefix...)
	t := event.Time
	flags := mode.Flags.Bits()
	if flags&(Ldate|Ltime|Lmicroseconds) != 0 {
		if mode.Colorize {
			buf = append(buf, fgCyanBytes...)
		}
		if flags&LUTC != 0 {
			t = t.UTC()
		}
		if flags&Ldate != 0 {
			year, month, day := t.Date()
			buf = itoa(buf, year, 4)
			buf = append(buf, '/')
			buf = itoa(buf, int(month), 2)
			buf = append(buf, '/')
			buf = itoa(buf, day, 2)
			buf = append(buf, ' ')
		}
		if flags&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			buf = itoa(buf, hour, 2)
			buf = append(buf, ':')
			buf = itoa(buf, min, 2)
			buf = append(buf, ':')
			buf = itoa(buf, sec, 2)
			if flags&Lmicroseconds != 0 {
				buf = append(buf, '.')
				buf = itoa(buf, t.Nanosecond()/1e3, 6)
			}
			buf = append(buf, ' ')
		}
		if mode.Colorize {
			buf = append(buf, resetBytes...)
		}

	}
	if flags&(Lshortfile|Llongfile) != 0 {
		if mode.Colorize {
			buf = append(buf, fgGreenBytes...)
		}
		file := event.Filename
		if flags&Lmedfile == Lmedfile {
			startIndex := len(file) - 20
			if startIndex > 0 {
				file = "..." + file[startIndex:]
			}
		} else if flags&Lshortfile != 0 {
			startIndex := strings.LastIndexByte(file, '/')
			if startIndex > 0 && startIndex < len(file) {
				file = file[startIndex+1:]
			}
		}
		buf = append(buf, file...)
		buf = append(buf, ':')
		buf = itoa(buf, event.Line, -1)
		if flags&(Lfuncname|Lshortfuncname) != 0 {
			buf = append(buf, ':')
		} else {
			if mode.Colorize {
				buf = append(buf, resetBytes...)
			}
			buf = append(buf, ' ')
		}
	}
	if flags&(Lfuncname|Lshortfuncname) != 0 {
		if mode.Colorize {
			buf = append(buf, fgGreenBytes...)
		}
		funcname := event.Caller
		if flags&Lshortfuncname != 0 {
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

	if flags&(Llevel|Llevelinitial) != 0 {
		level := strings.ToUpper(event.Level.String())
		if mode.Colorize {
			buf = append(buf, ColorBytes(levelToColor[event.Level]...)...)
		}
		buf = append(buf, '[')
		if flags&Llevelinitial != 0 {
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

	var msg []byte

	// if the log needs colorizing, do it
	if mode.Colorize && len(msgArgs) > 0 {
		hasColorValue := false
		for _, v := range msgArgs {
			if _, hasColorValue = v.(*ColoredValue); hasColorValue {
				break
			}
		}
		if hasColorValue {
			msg = []byte(fmt.Sprintf(msgFormat, msgArgs...))
		}
	}
	// try to re-use the pre-formatted simple text message
	if len(msg) == 0 {
		msg = []byte(event.MsgSimpleText)
	}
	// if still no message, do the normal Sprintf for the message
	if len(msg) == 0 {
		msg = []byte(colorSprintf(mode.Colorize, msgFormat, msgArgs...))
	}
	// remove at most one trailing new line
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	if flags&Lgopid == Lgopid {
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
		for _, line := range lines {
			buf = append(buf, "\n\t"...)
			buf = append(buf, line...)
		}
		buf = append(buf, '\n')
	}
	buf = append(buf, '\n')
	return buf
}
