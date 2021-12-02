// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package perm

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
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

// ColorFormat provides a ColorFormatted version of this AccessMode
func (mode AccessMode) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s",
		log.NewColoredIDValue(mode),
		mode)
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
