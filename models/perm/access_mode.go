// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package perm

import (
	"fmt"
)

// AccessMode specifies the users access mode
type AccessMode int

const (
	// AccessModeNone no access
	AccessModeNone AccessMode = iota // 0
	// AccessModeRead read access
	AccessModeRead // 1
	// AccessModeWrite write access
	AccessModeWrite // 2
	// AccessModeAdmin admin access
	AccessModeAdmin // 3
	// AccessModeOwner owner access
	AccessModeOwner // 4
)

func (mode AccessMode) String() string {
	switch mode {
	case AccessModeRead:
		return "read"
	case AccessModeWrite:
		return "write"
	case AccessModeAdmin:
		return "admin"
	case AccessModeOwner:
		return "owner"
	default:
		return "none"
	}
}

func (mode AccessMode) LogString() string {
	return fmt.Sprintf("<AccessMode:%d:%s>", mode, mode.String())
}

// ParseAccessMode returns corresponding access mode to given permission string.
func ParseAccessMode(permission string) AccessMode {
	switch permission {
	case "read":
		return AccessModeRead
	case "write":
		return AccessModeWrite
	case "admin":
		return AccessModeAdmin
	default:
		return AccessModeNone
	}
}
