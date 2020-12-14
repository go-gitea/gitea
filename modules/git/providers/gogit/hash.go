// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"encoding/hex"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/go-git/go-git/v5/plumbing"
)

var _ (service.Hash) = SHA1([20]byte{})
var _ (service.Hash) = StringHash("")

type plumbingHashable interface {
	ToPlumbingHash() plumbing.Hash
	FromPlumbingHash(hash plumbing.Hash) service.Hash
}

// ToPlumbingHash converts a provided Hash to a plumbing.Hash
func ToPlumbingHash(hash service.Hash) plumbing.Hash {
	ph, ok := hash.(plumbingHashable)
	if ok {
		return ph.ToPlumbingHash()
	}
	nativeSha1, ok := hash.(native.SHA1)
	if ok {
		return plumbing.Hash(nativeSha1)
	}

	s, err := SHA1{}.FromString(hash.String())
	if err != nil {
		return plumbing.Hash{}
	}
	return s.(SHA1).ToPlumbingHash()
}

// FromPlumbingHashes converts arrays of plumbing.Hash to service.Hash
func FromPlumbingHashes(hashes []plumbing.Hash) []service.Hash {
	ret := make([]service.Hash, len(hashes))
	for i, h := range hashes {
		ret[i] = fromPlumbingHash(h)
	}
	return ret
}

// FromPlumbingHash converts plumbing.Hash to service.Hash
func fromPlumbingHash(hash plumbing.Hash) service.Hash {
	return SHA1(hash)
}

// SHA1 is a 20 byte git hash
type SHA1 [20]byte

// String returns a string representation of the SHA
func (s SHA1) String() string {
	return hex.EncodeToString(s[:])
}

// ToPlumbingHash converts a hash to a plumbing.Hash
func (s SHA1) ToPlumbingHash() plumbing.Hash {
	return plumbing.Hash(s)
}

// FromPlumbingHash converts a hash to a SHA1
func (s SHA1) FromPlumbingHash(hash plumbing.Hash) service.Hash {
	s = SHA1(hash)
	return SHA1(hash)
}

// IsZero returns whether provided hash is zero
func (s SHA1) IsZero() bool {
	var empty SHA1
	return s == empty
}

// FromString converts a provided string to a new SHA1
func (s SHA1) FromString(idStr string) (service.Hash, error) {
	idStr = strings.TrimSpace(idStr)
	if len(idStr) != 40 {
		return s, fmt.Errorf("Length must be 40: %s", s)
	}
	b, err := hex.DecodeString(idStr)
	if err != nil {
		return s, err
	}
	copy(s[:], b)
	if len(b) != 20 {
		return SHA1{}, fmt.Errorf("Length must be 20: %v", b)
	}
	return s, nil
}

// MustIDFromString converts a string to hash
func MustIDFromString(idStr string) service.Hash {
	s, _ := SHA1{}.FromString(idStr)
	return s
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
	if service.SHAPattern.MatchString(idStr) {
		return StringHash(""), fmt.Errorf("String must match ^[0-9a-f]{4,40}$: %s", s)
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

// ToPlumbingHash converts to a plumbing.Hash
func (s StringHash) ToPlumbingHash() plumbing.Hash {
	return plumbing.Hash(s.SHA1())
}
