// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_parseTagData(t *testing.T) {
	testData := []struct {
		data []byte
		tag  Tag
	}{
		{data: []byte(`object 3b114ab800c6432ad42387ccf6bc8d4388a2885a
type commit
tag 1.22.0
tagger Lucas Michot <lucas@semalead.com> 1484491741 +0100

`), tag: Tag{
			Name:      "",
			ID:        SHA1{},
			Object:    SHA1{0x3b, 0x11, 0x4a, 0xb8, 0x0, 0xc6, 0x43, 0x2a, 0xd4, 0x23, 0x87, 0xcc, 0xf6, 0xbc, 0x8d, 0x43, 0x88, 0xa2, 0x88, 0x5a},
			Type:      "commit",
			Tagger:    &Signature{Name: "Lucas Michot", Email: "lucas@semalead.com", When: time.Unix(1484491741, 0)},
			Message:   "",
			Signature: nil,
		}},
		{data: []byte(`object 7cdf42c0b1cc763ab7e4c33c47a24e27c66bfccc
type commit
tag 1.22.1
tagger Lucas Michot <lucas@semalead.com> 1484553735 +0100

test message
o

ono`), tag: Tag{
			Name:      "",
			ID:        SHA1{},
			Object:    SHA1{0x7c, 0xdf, 0x42, 0xc0, 0xb1, 0xcc, 0x76, 0x3a, 0xb7, 0xe4, 0xc3, 0x3c, 0x47, 0xa2, 0x4e, 0x27, 0xc6, 0x6b, 0xfc, 0xcc},
			Type:      "commit",
			Tagger:    &Signature{Name: "Lucas Michot", Email: "lucas@semalead.com", When: time.Unix(1484553735, 0)},
			Message:   "test message\no\n\nono",
			Signature: nil,
		}},
	}

	for _, test := range testData {
		tag, err := parseTagData(test.data)
		assert.NoError(t, err)
		assert.EqualValues(t, test.tag.ID, tag.ID)
		assert.EqualValues(t, test.tag.Object, tag.Object)
		assert.EqualValues(t, test.tag.Name, tag.Name)
		assert.EqualValues(t, test.tag.Message, tag.Message)
		assert.EqualValues(t, test.tag.Type, tag.Type)
		if test.tag.Signature != nil && assert.NotNil(t, tag.Signature) {
			assert.EqualValues(t, test.tag.Signature.Signature, tag.Signature.Signature)
			assert.EqualValues(t, test.tag.Signature.Payload, tag.Signature.Payload)
		} else {
			assert.Nil(t, tag.Signature)
		}
		if test.tag.Tagger != nil && assert.NotNil(t, tag.Tagger) {
			assert.EqualValues(t, test.tag.Tagger.Name, tag.Tagger.Name)
			assert.EqualValues(t, test.tag.Tagger.Email, tag.Tagger.Email)
			assert.EqualValues(t, test.tag.Tagger.When.Unix(), tag.Tagger.When.Unix())
		} else {
			assert.Nil(t, tag.Tagger)
		}
	}
}
