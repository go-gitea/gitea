// +build windows

package xid

import (
	"fmt"
	"syscall"
	"unsafe"
)

func readPlatformMachineID() (string, error) {
	// source: https://github.com/shirou/gopsutil/blob/master/host/host_syscall.go
	var h syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, syscall.StringToUTF16Ptr(`SOFTWARE\Microsoft\Cryptography`), 0, syscall.KEY_READ|syscall.KEY_WOW64_64KEY, &h)
	if err != nil {
		return "", err
	}
	defer syscall.RegCloseKey(h)

	const syscallRegBufLen = 74 // len(`{`) + len(`abcdefgh-1234-456789012-123345456671` * 2) + len(`}`) // 2 == bytes/UTF16
	const uuidLen = 36

	var regBuf [syscallRegBufLen]uint16
	bufLen := uint32(syscallRegBufLen)
	var valType uint32
	err = syscall.RegQueryValueEx(h, syscall.StringToUTF16Ptr(`MachineGuid`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
	if err != nil {
		return "", err
	}

	hostID := syscall.UTF16ToString(regBuf[:])
	hostIDLen := len(hostID)
	if hostIDLen != uuidLen {
		return "", fmt.Errorf("HostID incorrect: %q\n", hostID)
	}

	return hostID, nil
}
