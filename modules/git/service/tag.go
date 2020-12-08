// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// Tag represents a Git tag.
type Tag interface {
	Object

	// Name returns the name of this tag
	Name() string

	// TagType returns the type of this tag
	TagType() string

	// TagObject returns the hash of target of this tag
	TagObject() Hash

	// Tagger returns the creator of the tag
	Tagger() *Signature

	// Message returns the message of the tag
	Message() string

	// Signature returns the GPG signature
	Signature() *GPGSignature
}
