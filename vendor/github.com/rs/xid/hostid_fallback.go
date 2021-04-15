// +build !darwin,!linux,!freebsd,!windows

package xid

import "errors"

func readPlatformMachineID() (string, error) {
	return "", errors.New("not implemented")
}
