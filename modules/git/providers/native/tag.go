// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/modules/git/service"
)

// BeginPGP is represents the start of a signature
const BeginPGP = "\n-----BEGIN PGP SIGNATURE-----\n"

// EndPGP is represents the end of a signature
const EndPGP = "\n-----END PGP SIGNATURE-----"

var _ (service.Tag) = &Tag{}

// Tag represents a Git tag.
type Tag struct {
	service.Object

	tagObject    service.Hash
	name         string
	tagType      string
	tagger       *service.Signature
	message      string
	gpgSignature *service.GPGSignature
}

// NewTag creates a tag
func NewTag(
	object service.Object,
	name string,
	tagObject service.Hash,
	tagType string,
	tagger *service.Signature,
	message string,
	signature *service.GPGSignature) service.Tag {
	return &Tag{
		Object:       object,
		name:         name,
		tagType:      tagType,
		tagger:       tagger,
		message:      message,
		gpgSignature: signature,
	}
}

// Name returns the name of this tag
func (t *Tag) Name() string {
	return t.name
}

// TagType returns the type of this tag
func (t *Tag) TagType() string {
	return t.tagType
}

// TagObject returns the object hash for this tag
func (t *Tag) TagObject() service.Hash {
	return t.tagObject
}

// Tagger returns the creator of the tag
func (t *Tag) Tagger() *service.Signature {
	return t.tagger
}

// Message returns the message of the tag
func (t *Tag) Message() string {
	return t.message
}

// Signature returns the GPG signature
func (t *Tag) Signature() *service.GPGSignature {
	return t.gpgSignature
}

// Parse commit information from the (uncompressed) raw
// data from the tag object.
// \n\n separate headers from message
func parseTagData(data []byte) (*Tag, error) {
	tag := &Tag{}
	// we now have the contents of the commit object. Let's investigate...
	nextline := 0
l:
	for {
		eol := bytes.IndexByte(data[nextline:], '\n')
		switch {
		case eol > 0:
			line := data[nextline : nextline+eol]
			spacepos := bytes.IndexByte(line, ' ')
			reftype := line[:spacepos]
			switch string(reftype) {
			case "object":
				id := StringHash(string(line[spacepos+1:]))

				tag.tagObject = id
			case "tag":
				tag.name = string(line[spacepos+1:])
			case "type":
				// A commit can have one or more parents
				tag.tagType = string(line[spacepos+1:])
			case "tagger":
				sig, err := service.NewSignatureFromCommitline(line[spacepos+1:])
				if err != nil {
					return nil, err
				}
				tag.tagger = sig
			}
			nextline += eol + 1
		case eol == 0:
			tag.message = string(data[nextline+1 : len(data)-1])
			break l
		default:
			break l
		}
	}
	idx := strings.LastIndex(tag.message, BeginPGP)
	if idx > 0 {
		endSigIdx := strings.Index(tag.message[idx:], EndPGP)
		if endSigIdx > 0 {
			tag.gpgSignature = &service.GPGSignature{
				Signature: tag.message[idx+1 : idx+endSigIdx+len(EndPGP)],
				Payload:   string(data[:bytes.LastIndex(data, []byte(BeginPGP))+1]),
			}
			tag.message = tag.message[:idx+1]
		}
	}
	return tag, nil
}
