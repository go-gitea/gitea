// +build !go1.3

package mssql

import (
	"net"
)

func createDialer(p *connectParams) *net.Dialer {
	return &net.Dialer{Timeout: p.dial_timeout}
}
