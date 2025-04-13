// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha1"
	"crypto/sha256"
	"regexp"
	"strconv"
)

// sha1Pattern can be used to determine if a string is an valid sha
var sha1Pattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// sha256Pattern can be used to determine if a string is an valid sha
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

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
	emptySha1ObjectID = &Sha1Hash{}
	emptySha1Tree     = &Sha1Hash{
		0x4b, 0x82, 0x5d, 0xc6, 0x42, 0xcb, 0x6e, 0xb9, 0xa0, 0x60,
		0xe5, 0x4b, 0xf8, 0xd6, 0x92, 0x88, 0xfb, 0xee, 0x49, 0x04,
	}
)

func (Sha1ObjectFormatImpl) Name() string { return "sha1" }
func (Sha1ObjectFormatImpl) EmptyObjectID() ObjectID {
	return emptySha1ObjectID
}

func (Sha1ObjectFormatImpl) EmptyTree() ObjectID {
	return emptySha1Tree
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
	_, _ = hasher.Write(content)
	return h.MustID(hasher.Sum(nil))
}

type Sha256ObjectFormatImpl struct{}

var (
	emptySha256ObjectID = &Sha256Hash{}
	emptySha256Tree     = &Sha256Hash{
		0x6e, 0xf1, 0x9b, 0x41, 0x22, 0x5c, 0x53, 0x69, 0xf1, 0xc1,
		0x04, 0xd4, 0x5d, 0x8d, 0x85, 0xef, 0xa9, 0xb0, 0x57, 0xb5,
		0x3b, 0x14, 0xb4, 0xb9, 0xb9, 0x39, 0xdd, 0x74, 0xde, 0xcc,
		0x53, 0x21,
	}
)

func (Sha256ObjectFormatImpl) Name() string { return "sha256" }
func (Sha256ObjectFormatImpl) EmptyObjectID() ObjectID {
	return emptySha256ObjectID
}

func (Sha256ObjectFormatImpl) EmptyTree() ObjectID {
	return emptySha256Tree
}
func (Sha256ObjectFormatImpl) FullLength() int { return 64 }
func (Sha256ObjectFormatImpl) IsValid(input string) bool {
	return sha256Pattern.MatchString(input)
}

func (Sha256ObjectFormatImpl) MustID(b []byte) ObjectID {
	var id Sha256Hash
	copy(id[0:32], b)
	return &id
}

// ComputeHash compute the hash for a given ObjectType and content
func (h Sha256ObjectFormatImpl) ComputeHash(t ObjectType, content []byte) ObjectID {
	hasher := sha256.New()
	_, _ = hasher.Write(t.Bytes())
	_, _ = hasher.Write([]byte(" "))
	_, _ = hasher.Write([]byte(strconv.FormatInt(int64(len(content)), 10)))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(content)
	return h.MustID(hasher.Sum(nil))
}

var (
	Sha1ObjectFormat   ObjectFormat = Sha1ObjectFormatImpl{}
	Sha256ObjectFormat ObjectFormat = Sha256ObjectFormatImpl{}
)

func ObjectFormatFromName(name string) ObjectFormat {
	for _, objectFormat := range DefaultFeatures().SupportedObjectFormats {
		if name == objectFormat.Name() {
			return objectFormat
		}
	}
	return nil
}

func IsValidObjectFormat(name string) bool {
	return ObjectFormatFromName(name) != nil
}
