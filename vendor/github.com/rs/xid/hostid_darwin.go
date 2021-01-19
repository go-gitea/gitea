// +build darwin

package xid

import "syscall"

func readPlatformMachineID() (string, error) {
	return syscall.Sysctl("kern.uuid")
}
