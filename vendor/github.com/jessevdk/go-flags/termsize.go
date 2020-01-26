// +build !windows,!plan9,!solaris,!appengine

package flags

import (
	"syscall"
	"unsafe"
)

type winsize struct {
	row, col       uint16
	xpixel, ypixel uint16
}

func getTerminalColumns() int {
	ws := winsize{}

	if tIOCGWINSZ != 0 {
		syscall.Syscall(syscall.SYS_IOCTL,
			uintptr(0),
			uintptr(tIOCGWINSZ),
			uintptr(unsafe.Pointer(&ws)))

		return int(ws.col)
	}

	return 80
}
