// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"
)

type ObjectFormatID int

const (
	Sha1 ObjectFormatID = iota
)

// sha1Pattern can be used to determine if a string is an valid sha
var sha1Pattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

type ObjectFormat interface {
	ID() ObjectFormatID
	String() string

	// Empty is the hash of empty git
	Empty() ObjectID
	// EmptyTree is the hash of an empty tree
	EmptyTree() ObjectID
	// FullLength is the length of the hash's hex string
	FullLength() int

	IsValid(input string) bool
	MustID(b []byte) ObjectID
	MustIDFromString(s string) ObjectID
	NewID(b []byte) (ObjectID, error)
	NewIDFromString(s string) (ObjectID, error)
	NewEmptyID() ObjectID

	NewHasher() HasherInterface
}

type Sha1ObjectFormat struct{}

func (*Sha1ObjectFormat) ID() ObjectFormatID { return Sha1 }
func (*Sha1ObjectFormat) String() string     { return "sha1" }
func (*Sha1ObjectFormat) Empty() ObjectID    { return &Sha1Hash{} }
func (*Sha1ObjectFormat) EmptyTree() ObjectID {
	return &Sha1Hash{
		0x4b, 0x82, 0x5d, 0xc6, 0x42, 0xcb, 0x6e, 0xb9, 0xa0, 0x60,
		0xe5, 0x4b, 0xf8, 0xd6, 0x92, 0x88, 0xfb, 0xee, 0x49, 0x04,
	}
}
func (*Sha1ObjectFormat) FullLength() int { return 40 }
func (*Sha1ObjectFormat) IsValid(input string) bool {
	return sha1Pattern.MatchString(input)
}

func (*Sha1ObjectFormat) MustID(b []byte) ObjectID {
	var id Sha1Hash
	copy(id[0:20], b)
	return &id
}

func (h *Sha1ObjectFormat) MustIDFromString(s string) ObjectID {
	return MustIDFromString(h, s)
}

func (h *Sha1ObjectFormat) NewID(b []byte) (ObjectID, error) {
	return IDFromRaw(h, b)
}

func (h *Sha1ObjectFormat) NewIDFromString(s string) (ObjectID, error) {
	return genericIDFromString(h, s)
}

func (*Sha1ObjectFormat) NewEmptyID() ObjectID {
	return NewSha1()
}

func (h *Sha1ObjectFormat) NewHasher() HasherInterface {
	return &Sha1Hasher{sha1.New()}
}

func ObjectFormatFromID(id ObjectFormatID) ObjectFormat {
	switch id {
	case Sha1:
		return &Sha1ObjectFormat{}
	}

	return nil
}

func ObjectFormatFromString(hash string) (ObjectFormat, error) {
	switch strings.ToLower(hash) {
	case "sha1":
		return &Sha1ObjectFormat{}, nil
	}

	return nil, fmt.Errorf("unknown hash type: %s", hash)
}
