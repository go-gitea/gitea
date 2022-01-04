// +build !windows,!plan9,!appengine,!wasm

package flags

import (
	"golang.org/x/sys/unix"
)

func getTerminalColumns() int {
	ws, err := unix.IoctlGetWinsize(0, unix.TIOCGWINSZ)
	if err != nil {
		return 80
	}
	return int(ws.Col)
}
