// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package perm

import (
	"fmt"
	"slices"

	"code.gitea.io/gitea/modules/util"
)

// AccessMode specifies the users access mode
type AccessMode int

const (
	AccessModeNone AccessMode = iota // 0: no access

	AccessModeRead  // 1: read access
	AccessModeWrite // 2: write access
	AccessModeAdmin // 3: admin access
	AccessModeOwner // 4: owner access
)

// ToString returns the string representation of the access mode, do not make it a Stringer, otherwise it's difficult to render in templates
func (mode AccessMode) ToString() string {
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
	return fmt.Sprintf("<AccessMode:%d:%s>", mode, mode.ToString())
}

// ParseAccessMode returns corresponding access mode to given permission string.
func ParseAccessMode(permission string, allowed ...AccessMode) AccessMode {
	m := AccessModeNone
	switch permission {
	case "read":
		m = AccessModeRead
	case "write":
		m = AccessModeWrite
	case "admin":
		m = AccessModeAdmin
	default:
		// the "owner" access is not really used for user input, it's mainly for checking access level in code, so don't parse it
	}
	if len(allowed) == 0 {
		return m
	}
	return util.Iif(slices.Contains(allowed, m), m, AccessModeNone)
}

// ErrInvalidAccessMode is returned when an invalid access mode is used
var ErrInvalidAccessMode = util.NewInvalidArgumentErrorf("Invalid access mode")
