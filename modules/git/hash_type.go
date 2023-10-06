// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

type Hash interface {
	String() string
	IsZero() bool
	HashType() HashType
	Bytes() []byte
}

type HashType interface {
	Empty() string
	EmptyTree() string
	FullLength() int
	IsValid(sha string) bool
	NewHashFromBytes(b []byte) Hash
	EmptyHash() Hash
	ComputeHash(t ObjectType, content []byte) Hash
}

func (repo *Repository) MustHashFromString(s string) Hash {
	return MustHashFromStringByType(repo.HashType, s)
}
