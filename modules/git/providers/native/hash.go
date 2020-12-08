// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"encoding/hex"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.Hash) = &SHA1{}
var _ (service.Hash) = StringHash("")

// SHA1 is a 20 byte git hash
type SHA1 [20]byte

// String returns a string representation of the SHA
func (s SHA1) String() string {
	return hex.EncodeToString(s[:])
}

// IsZero returns whether provided hash is zero
func (s SHA1) IsZero() bool {
	var empty SHA1
	return s == empty
}

// FromString converts a provided string to a new SHA1
func (s SHA1) FromString(idStr string) (service.Hash, error) {
	var id SHA1
	idStr = strings.TrimSpace(idStr)
	if len(s) != 40 {
		return id, fmt.Errorf("Length must be 40: %s", s)
	}
	b, err := hex.DecodeString(idStr)
	if err != nil {
		return id, err
	}
	copy(id[:], b)
	if len(b) != 20 {
		return SHA1{}, fmt.Errorf("Length must be 20: %v", b)
	}
	return id, nil
}

// StringHash represents a hash from a string
type StringHash string

// IsZero returns whether provided hash is zero
func (s StringHash) IsZero() bool {
	if s == "" || s == service.EmptySHA {
		return true
	}
	return false
}

// String returns the string value for this hash
func (s StringHash) String() string {
	if s == "" {
		return service.EmptySHA
	}
	return string(s)
}

// Valid asserts that the provided string hash is a potentially valid hash
func (s StringHash) Valid() bool {
	return service.SHAPattern.MatchString(s.String())
}

// FromString converts a provided string to a new SHA1
func (s StringHash) FromString(idStr string) (service.Hash, error) {
	idStr = strings.TrimSpace(idStr)
	if !service.SHAPattern.MatchString(idStr) {
		return StringHash(""), fmt.Errorf("String must match ^[0-9a-f]{4,40}$: %q", idStr)
	}
	return StringHash(idStr), nil
}

// SHA1 converts a string hash to SHA1 byte hash
func (s StringHash) SHA1() SHA1 {
	var id SHA1
	b, _ := hex.DecodeString(s.String())
	copy(id[:], b)
	return id
}
