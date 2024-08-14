// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

type ObjectID interface {
	String() string
	IsZero() bool
	RawValue() []byte
	Type() ObjectFormat
}

type Sha1Hash [20]byte

func (h *Sha1Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h *Sha1Hash) IsZero() bool {
	empty := Sha1Hash{}
	return bytes.Equal(empty[:], h[:])
}
func (h *Sha1Hash) RawValue() []byte { return h[:] }
func (*Sha1Hash) Type() ObjectFormat { return Sha1ObjectFormat }

var _ ObjectID = &Sha1Hash{}

func MustIDFromString(hexHash string) ObjectID {
	id, err := NewIDFromString(hexHash)
	if err != nil {
		panic(err)
	}
	return id
}

type Sha256Hash [32]byte

func (h *Sha256Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h *Sha256Hash) IsZero() bool {
	empty := Sha256Hash{}
	return bytes.Equal(empty[:], h[:])
}
func (h *Sha256Hash) RawValue() []byte { return h[:] }
func (*Sha256Hash) Type() ObjectFormat { return Sha256ObjectFormat }

func NewIDFromString(hexHash string) (ObjectID, error) {
	var theObjectFormat ObjectFormat
	for _, objectFormat := range DefaultFeatures().SupportedObjectFormats {
		if len(hexHash) == objectFormat.FullLength() {
			theObjectFormat = objectFormat
			break
		}
	}

	if theObjectFormat == nil {
		return nil, fmt.Errorf("length %d has no matched object format: %s", len(hexHash), hexHash)
	}

	b, err := hex.DecodeString(hexHash)
	if err != nil {
		return nil, err
	}

	if len(b) != theObjectFormat.FullLength()/2 {
		return theObjectFormat.EmptyObjectID(), fmt.Errorf("length must be %d: %v", theObjectFormat.FullLength(), b)
	}
	return theObjectFormat.MustID(b), nil
}

func IsEmptyCommitID(commitID string) bool {
	if commitID == "" {
		return true
	}

	id, err := NewIDFromString(commitID)
	if err != nil {
		return false
	}

	return id.IsZero()
}

// ComputeBlobHash compute the hash for a given blob content
func ComputeBlobHash(hashType ObjectFormat, content []byte) ObjectID {
	return hashType.ComputeHash(ObjectBlob, content)
}

type ErrInvalidSHA struct {
	SHA string
}

func (err ErrInvalidSHA) Error() string {
	return fmt.Sprintf("invalid sha: %s", err.SHA)
}
