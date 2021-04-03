// Code generated by cmd/cgo -godefs; DO NOT EDIT.
// cgo -godefs defs_aix.go

// Added for go1.11 compatibility
//go:build aix
// +build aix

package ipv6

const (
	sysIPV6_PATHMTU  = 0x2e
	sysIPV6_PKTINFO  = 0x21
	sysIPV6_HOPLIMIT = 0x28
	sysIPV6_NEXTHOP  = 0x30
	sysIPV6_TCLASS   = 0x2b

	sizeofSockaddrStorage = 0x508
	sizeofSockaddrInet6   = 0x1c
	sizeofInet6Pktinfo    = 0x14
	sizeofIPv6Mtuinfo     = 0x20

	sizeofIPv6Mreq       = 0x14
	sizeofGroupReq       = 0x510
	sizeofGroupSourceReq = 0xa18

	sizeofICMPv6Filter = 0x20
)

type sockaddrStorage struct {
	X__ss_len   uint8
	Family      uint8
	X__ss_pad1  [6]uint8
	X__ss_align int64
	X__ss_pad2  [1265]uint8
	Pad_cgo_0   [7]byte
}

type sockaddrInet6 struct {
	Len      uint8
	Family   uint8
	Port     uint16
	Flowinfo uint32
	Addr     [16]byte /* in6_addr */
	Scope_id uint32
}

type inet6Pktinfo struct {
	Addr    [16]byte /* in6_addr */
	Ifindex int32
}

type ipv6Mtuinfo struct {
	Addr sockaddrInet6
	Mtu  uint32
}

type ipv6Mreq struct {
	Multiaddr [16]byte /* in6_addr */
	Interface uint32
}

type icmpv6Filter struct {
	Filt [8]uint32
}

type groupReq struct {
	Interface uint32
	Group     sockaddrStorage
}

type groupSourceReq struct {
	Interface uint32
	Group     sockaddrStorage
	Source    sockaddrStorage
}
