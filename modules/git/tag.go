// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"sort"

	"code.gitea.io/gitea/modules/util"
)

// Tag represents a Git tag.
type Tag struct {
	Name      string
	ID        ObjectID
	Object    ObjectID // The id of this commit object
	Type      string
	Tagger    *Signature
	Message   string
	Signature *CommitSignature
}

func parsePayloadSignature(data []byte, messageStart int) (payload, msg, sign string) {
	pos := messageStart
	signStart, signEnd := -1, -1
	for {
		eol := bytes.IndexByte(data[pos:], '\n')
		if eol < 0 {
			break
		}
		line := data[pos : pos+eol]
		signType, hasPrefix := bytes.CutPrefix(line, []byte("-----BEGIN "))
		signType, hasSuffix := bytes.CutSuffix(signType, []byte(" SIGNATURE-----"))
		if hasPrefix && hasSuffix {
			signEndBytes := append([]byte("\n-----END "), signType...)
			signEndBytes = append(signEndBytes, []byte(" SIGNATURE-----")...)
			signEnd = bytes.Index(data[pos:], signEndBytes)
			if signEnd != -1 {
				signStart = pos
				signEnd = pos + signEnd + len(signEndBytes)
			}
		}
		pos += eol + 1
	}

	if signStart != -1 && signEnd != -1 {
		msgEnd := max(messageStart, signStart-1)
		return string(data[:msgEnd]), string(data[messageStart:msgEnd]), string(data[signStart:signEnd])
	}
	return string(data), string(data[messageStart:]), ""
}

// Parse commit information from the (uncompressed) raw
// data from the commit object.
// \n\n separate headers from message
func parseTagData(objectFormat ObjectFormat, data []byte) (*Tag, error) {
	tag := new(Tag)
	tag.ID = objectFormat.EmptyObjectID()
	tag.Object = objectFormat.EmptyObjectID()
	tag.Tagger = &Signature{}

	pos := 0
	for {
		eol := bytes.IndexByte(data[pos:], '\n')
		if eol == -1 {
			break // shouldn't happen, but could just tolerate it
		}
		if eol == 0 {
			pos++
			break // end of headers
		}
		line := data[pos : pos+eol]
		key, val, _ := bytes.Cut(line, []byte(" "))
		switch string(key) {
		case "object":
			id, err := NewIDFromString(string(val))
			if err != nil {
				return nil, err
			}
			tag.Object = id
		case "type":
			tag.Type = string(val) // A commit can have one or more parents
		case "tagger":
			tag.Tagger = parseSignatureFromCommitLine(util.UnsafeBytesToString(val))
		}
		pos += eol + 1
	}
	payload, msg, sign := parsePayloadSignature(data, pos)
	tag.Message = msg
	if len(sign) > 0 {
		tag.Signature = &CommitSignature{Signature: sign, Payload: payload}
	}
	return tag, nil
}

type tagSorter []*Tag

func (ts tagSorter) Len() int {
	return len([]*Tag(ts))
}

func (ts tagSorter) Less(i, j int) bool {
	return []*Tag(ts)[i].Tagger.When.After([]*Tag(ts)[j].Tagger.When)
}

func (ts tagSorter) Swap(i, j int) {
	[]*Tag(ts)[i], []*Tag(ts)[j] = []*Tag(ts)[j], []*Tag(ts)[i]
}

// sortTagsByTime
func sortTagsByTime(tags []*Tag) {
	sorter := tagSorter(tags)
	sort.Sort(sorter)
}
