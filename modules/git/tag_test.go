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
		data     string
		expected Tag
	}{
		{
			data: `object 3b114ab800c6432ad42387ccf6bc8d4388a2885a
type commit
tag 1.22.0
tagger Lucas Michot <lucas@semalead.com> 1484491741 +0100

`,
			expected: Tag{
				Name:      "",
				ID:        Sha1ObjectFormat.EmptyObjectID(),
				Object:    MustIDFromString("3b114ab800c6432ad42387ccf6bc8d4388a2885a"),
				Type:      "commit",
				Tagger:    &Signature{Name: "Lucas Michot", Email: "lucas@semalead.com", When: time.Unix(1484491741, 0).In(time.FixedZone("", 3600))},
				Message:   "",
				Signature: nil,
			},
		},
		{
			data: `object 7cdf42c0b1cc763ab7e4c33c47a24e27c66bfccc
type commit
tag 1.22.1
tagger Lucas Michot <lucas@semalead.com> 1484553735 +0100

test message
o

ono`,
			expected: Tag{
				Name:      "",
				ID:        Sha1ObjectFormat.EmptyObjectID(),
				Object:    MustIDFromString("7cdf42c0b1cc763ab7e4c33c47a24e27c66bfccc"),
				Type:      "commit",
				Tagger:    &Signature{Name: "Lucas Michot", Email: "lucas@semalead.com", When: time.Unix(1484553735, 0).In(time.FixedZone("", 3600))},
				Message:   "test message\no\n\nono",
				Signature: nil,
			},
		},
		{
			data: `object 7cdf42c0b1cc763ab7e4c33c47a24e27c66bfaaa
type commit
tag v0
tagger dummy user <dummy-email@example.com> 1484491741 +0100

dummy message
-----BEGIN SSH SIGNATURE-----
dummy signature
-----END SSH SIGNATURE-----
`,
			expected: Tag{
				Name:    "",
				ID:      Sha1ObjectFormat.EmptyObjectID(),
				Object:  MustIDFromString("7cdf42c0b1cc763ab7e4c33c47a24e27c66bfaaa"),
				Type:    "commit",
				Tagger:  &Signature{Name: "dummy user", Email: "dummy-email@example.com", When: time.Unix(1484491741, 0).In(time.FixedZone("", 3600))},
				Message: "dummy message",
				Signature: &CommitSignature{
					Signature: `-----BEGIN SSH SIGNATURE-----
dummy signature
-----END SSH SIGNATURE-----`,
					Payload: `object 7cdf42c0b1cc763ab7e4c33c47a24e27c66bfaaa
type commit
tag v0
tagger dummy user <dummy-email@example.com> 1484491741 +0100

dummy message`,
				},
			},
		},
	}

	for _, test := range testData {
		tag, err := parseTagData(Sha1ObjectFormat, []byte(test.data))
		assert.NoError(t, err)
		assert.Equal(t, test.expected, *tag)
	}

	tag, err := parseTagData(Sha1ObjectFormat, []byte("type commit\n\nfoo\n-----BEGIN SSH SIGNATURE-----\ncorrupted..."))
	assert.NoError(t, err)
	assert.Equal(t, "foo\n-----BEGIN SSH SIGNATURE-----\ncorrupted...", tag.Message)
}
