package transfer

import (
	"fmt"
	"path"
	"regexp"
)

// Pointer is a Git LFS pointer.
type Pointer struct {
	Oid  string `json:"oid"`
	Size int64  `json:"size"`
}

// String returns the string representation of the pointer.
func (p Pointer) String() string {
	return fmt.Sprintf("%s %d", p.Oid, p.Size)
}

var oidPattern = regexp.MustCompile(`^[a-f\d]{64}$`)

// IsValid checks if the pointer has a valid structure.
// It doesn't check if the pointed-to-content exists.
func (p Pointer) IsValid() bool {
	if len(p.Oid) != 64 {
		return false
	}
	if !oidPattern.MatchString(p.Oid) {
		return false
	}
	if p.Size < 0 {
		return false
	}
	return true
}

// RelativePath returns the relative storage path of the pointer.
func (p Pointer) RelativePath() string {
	if len(p.Oid) < 5 {
		return p.Oid
	}

	return path.Join(p.Oid[0:2], p.Oid[2:4], p.Oid)
}
