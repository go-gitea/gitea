// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

type Hash interface {
	String() string
	IsZero() bool
	HashType() HashType
	Bytes() []byte // TODO: remove this interface when it will not be used.
}

type HashType interface {
	Empty() string
	EmptyTree() string
	FullLength() int
	IsValid(sha string) bool
	NewHashFromBytes(b []byte) Hash
	ComputeHash(t ObjectType, content []byte) Hash
}

func (repo *Repository) MustHashFromString(s string) Hash {
	return MustHashFromStringByType(repo.HashType, s)
}

func HashTypeFromExample(commitID string) HashType {
	if len(commitID) == SHA1FullLength {
		return Sha1HashType{}
	}
	return Sha256HashType{}
}
