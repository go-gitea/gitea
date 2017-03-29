// +build freebsd openbsd netbsd dragonfly linux

package log

import (
	"log"
	"os"
	"syscall"
)

func CrashLog(file string) {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Println(err.Error())
	} else {
		syscall.Dup3(int(f.Fd()), 2, 0)
	}
}
