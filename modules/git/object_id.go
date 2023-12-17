// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
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

// EmptyObjectID creates a new ObjectID from an object format hash name
func EmptyObjectID(objectFormatName string) (ObjectID, error) {
	objectFormat := ObjectFormatFromName(objectFormatName)
	if objectFormat != nil {
		return objectFormat.EmptyObjectID(), nil
	}

	return nil, errors.New("unsupported hash type")
}

func IDFromRaw(h ObjectFormat, b []byte) (ObjectID, error) {
	if len(b) != h.FullLength()/2 {
		return h.EmptyObjectID(), fmt.Errorf("length must be %d: %v", h.FullLength(), b)
	}
	return h.MustID(b), nil
}

func MustIDFromString(h ObjectFormat, s string) ObjectID {
	b, _ := hex.DecodeString(s)
	return h.MustID(b)
}

func genericIDFromString(h ObjectFormat, s string) (ObjectID, error) {
	s = strings.TrimSpace(s)
	if len(s) != h.FullLength() {
		return h.EmptyObjectID(), fmt.Errorf("length must be %d: %s", h.FullLength(), s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return h.EmptyObjectID(), err
	}
	return h.NewID(b)
}

func IDFromString(hexHash string) (ObjectID, error) {
	for _, objectFormat := range SupportedObjectFormats {
		if len(hexHash) == objectFormat.FullLength() {
			return objectFormat.NewIDFromString(hexHash)
		}
	}

	return nil, fmt.Errorf("invalid hash hex string: '%s' len: %d", hexHash, len(hexHash))
}

func IsEmptyCommitID(commitID string) bool {
	if commitID == "" {
		return true
	}

	id, err := IDFromString(commitID)
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
