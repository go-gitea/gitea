// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha1"
	"regexp"
	"strconv"
)

// sha1Pattern can be used to determine if a string is an valid sha
var sha1Pattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

type ObjectFormat interface {
	// Name returns the name of the object format
	Name() string
	// EmptyObjectID creates a new empty ObjectID from an object format hash name
	EmptyObjectID() ObjectID
	// EmptyTree is the hash of an empty tree
	EmptyTree() ObjectID
	// FullLength is the length of the hash's hex string
	FullLength() int
	// IsValid returns true if the input is a valid hash
	IsValid(input string) bool
	// MustID creates a new ObjectID from a byte slice
	MustID(b []byte) ObjectID
	// ComputeHash compute the hash for a given ObjectType and content
	ComputeHash(t ObjectType, content []byte) ObjectID
}

type Sha1ObjectFormatImpl struct{}

var (
	emptyObjectID = &Sha1Hash{}
	emptyTree     = &Sha1Hash{
		0x4b, 0x82, 0x5d, 0xc6, 0x42, 0xcb, 0x6e, 0xb9, 0xa0, 0x60,
		0xe5, 0x4b, 0xf8, 0xd6, 0x92, 0x88, 0xfb, 0xee, 0x49, 0x04,
	}
)

func (Sha1ObjectFormatImpl) Name() string { return "sha1" }
func (Sha1ObjectFormatImpl) EmptyObjectID() ObjectID {
	return emptyObjectID
}

func (Sha1ObjectFormatImpl) EmptyTree() ObjectID {
	return emptyTree
}
func (Sha1ObjectFormatImpl) FullLength() int { return 40 }
func (Sha1ObjectFormatImpl) IsValid(input string) bool {
	return sha1Pattern.MatchString(input)
}

func (Sha1ObjectFormatImpl) MustID(b []byte) ObjectID {
	var id Sha1Hash
	copy(id[0:20], b)
	return &id
}

// ComputeHash compute the hash for a given ObjectType and content
func (h Sha1ObjectFormatImpl) ComputeHash(t ObjectType, content []byte) ObjectID {
	hasher := sha1.New()
	_, _ = hasher.Write(t.Bytes())
	_, _ = hasher.Write([]byte(" "))
	_, _ = hasher.Write([]byte(strconv.FormatInt(int64(len(content)), 10)))
	_, _ = hasher.Write([]byte{0})

	// HashSum generates a SHA1 for the provided hash
	var sha1 Sha1Hash
	copy(sha1[:], hasher.Sum(nil))
	return &sha1
}

var Sha1ObjectFormat ObjectFormat = Sha1ObjectFormatImpl{}

var SupportedObjectFormats = []ObjectFormat{
	Sha1ObjectFormat,
	// TODO: add sha256
}

func ObjectFormatFromName(name string) ObjectFormat {
	for _, objectFormat := range SupportedObjectFormats {
		if name == objectFormat.Name() {
			return objectFormat
		}
	}
	return nil
}

func IsValidObjectFormat(name string) bool {
	return ObjectFormatFromName(name) != nil
}
