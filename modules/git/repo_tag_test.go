// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetTags(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := openRepositoryWithDefaultContext(bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer bareRepo1.Close()

	tags, total, err := bareRepo1.GetTagInfos(0, 0)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.Len(t, tags, 2)
	assert.Len(t, tags, total)
	assert.EqualValues(t, "signed-tag", tags[0].Name)
	assert.EqualValues(t, "36f97d9a96457e2bab511db30fe2db03893ebc64", tags[0].ID.String())
	assert.EqualValues(t, "tag", tags[0].Type)
	assert.EqualValues(t, "test", tags[1].Name)
	assert.EqualValues(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", tags[1].ID.String())
	assert.EqualValues(t, "tag", tags[1].Type)
}

func TestRepository_GetTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	bareRepo1, err := openRepositoryWithDefaultContext(clonedPath)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer bareRepo1.Close()

	// LIGHTWEIGHT TAGS
	lTagCommitID := "6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1"
	lTagName := "lightweightTag"

	// Create the lightweight tag
	err = bareRepo1.CreateTag(lTagName, lTagCommitID)
	if err != nil {
		assert.NoError(t, err, "Unable to create the lightweight tag: %s for ID: %s. Error: %v", lTagName, lTagCommitID, err)
		return
	}

	// and try to get the Tag for lightweight tag
	lTag, err := bareRepo1.GetTag(lTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	if lTag == nil {
		assert.NotNil(t, lTag)
		assert.FailNow(t, "nil lTag: %s", lTagName)
	}
	assert.EqualValues(t, lTagName, lTag.Name)
	assert.EqualValues(t, lTagCommitID, lTag.ID.String())
	assert.EqualValues(t, lTagCommitID, lTag.Object.String())
	assert.EqualValues(t, "commit", lTag.Type)

	// ANNOTATED TAGS
	aTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	aTagName := "annotatedTag"
	aTagMessage := "my annotated message \n - test two line"

	// Create the annotated tag
	err = bareRepo1.CreateAnnotatedTag(aTagName, aTagMessage, aTagCommitID)
	if err != nil {
		assert.NoError(t, err, "Unable to create the annotated tag: %s for ID: %s. Error: %v", aTagName, aTagCommitID, err)
		return
	}

	// Now try to get the tag for the annotated Tag
	aTagID, err := bareRepo1.GetTagID(aTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	aTag, err := bareRepo1.GetTag(aTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	if aTag == nil {
		assert.NotNil(t, aTag)
		assert.FailNow(t, "nil aTag: %s", aTagName)
	}
	assert.EqualValues(t, aTagName, aTag.Name)
	assert.EqualValues(t, aTagID, aTag.ID.String())
	assert.NotEqual(t, aTagID, aTag.Object.String())
	assert.EqualValues(t, aTagCommitID, aTag.Object.String())
	assert.EqualValues(t, "tag", aTag.Type)

	// RELEASE TAGS

	rTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	rTagName := "release/" + lTagName

	err = bareRepo1.CreateTag(rTagName, rTagCommitID)
	if err != nil {
		assert.NoError(t, err, "Unable to create the  tag: %s for ID: %s. Error: %v", rTagName, rTagCommitID, err)
		return
	}

	rTagID, err := bareRepo1.GetTagID(rTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.EqualValues(t, rTagCommitID, rTagID)

	oTagID, err := bareRepo1.GetTagID(lTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.EqualValues(t, lTagCommitID, oTagID)
}

func TestRepository_GetAnnotatedTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	bareRepo1, err := openRepositoryWithDefaultContext(clonedPath)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer bareRepo1.Close()

	lTagCommitID := "6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1"
	lTagName := "lightweightTag"
	bareRepo1.CreateTag(lTagName, lTagCommitID)

	aTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	aTagName := "annotatedTag"
	aTagMessage := "my annotated message"
	bareRepo1.CreateAnnotatedTag(aTagName, aTagMessage, aTagCommitID)
	aTagID, _ := bareRepo1.GetTagID(aTagName)

	// Try an annotated tag
	tag, err := bareRepo1.GetAnnotatedTag(aTagID)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.NotNil(t, tag)
	assert.EqualValues(t, aTagName, tag.Name)
	assert.EqualValues(t, aTagID, tag.ID.String())
	assert.EqualValues(t, "tag", tag.Type)

	// Annotated tag's Commit ID should fail
	tag2, err := bareRepo1.GetAnnotatedTag(aTagCommitID)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
	assert.Nil(t, tag2)

	// Annotated tag's name should fail
	tag3, err := bareRepo1.GetAnnotatedTag(aTagName)
	assert.Error(t, err)
	assert.Errorf(t, err, "Length must be 40: %d", len(aTagName))
	assert.Nil(t, tag3)

	// Lightweight Tag should fail
	tag4, err := bareRepo1.GetAnnotatedTag(lTagCommitID)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
	assert.Nil(t, tag4)
}

func TestRepository_parseTagRef(t *testing.T) {
	tests := []struct {
		name string

		givenRef map[string]string

		want        *Tag
		wantErr     bool
		expectedErr error
	}{
		{
			name: "lightweight tag",

			givenRef: map[string]string{
				"objecttype":    "commit",
				"refname:short": "v1.9.1",
				// object will be empty for lightweight tags
				"object":     "",
				"objectname": "ab23e4b7f4cd0caafe0174c0e7ef6d651ba72889",
				"creator":    "Foo Bar <foo@bar.com> 1565789218 +0300",
				"contents": `Add changelog of v1.9.1 (#7859)

* add changelog of v1.9.1
* Update CHANGELOG.md
`,
				"contents:signature": "",
			},

			want: &Tag{
				Name:      "v1.9.1",
				ID:        MustIDFromString("ab23e4b7f4cd0caafe0174c0e7ef6d651ba72889"),
				Object:    MustIDFromString("ab23e4b7f4cd0caafe0174c0e7ef6d651ba72889"),
				Type:      "commit",
				Tagger:    parseAuthorLine(t, "Foo Bar <foo@bar.com> 1565789218 +0300"),
				Message:   "Add changelog of v1.9.1 (#7859)\n\n* add changelog of v1.9.1\n* Update CHANGELOG.md\n",
				Signature: nil,
			},
		},

		{
			name: "annotated tag",

			givenRef: map[string]string{
				"objecttype":    "tag",
				"refname:short": "v0.0.1",
				// object will refer to commit hash for annotated tag
				"object":     "3325fd8a973321fd59455492976c042dde3fd1ca",
				"objectname": "8c68a1f06fc59c655b7e3905b159d761e91c53c9",
				"creator":    "Foo Bar <foo@bar.com> 1565789218 +0300",
				"contents": `Add changelog of v1.9.1 (#7859)

* add changelog of v1.9.1
* Update CHANGELOG.md
`,
				"contents:signature": "",
			},

			want: &Tag{
				Name:      "v0.0.1",
				ID:        MustIDFromString("8c68a1f06fc59c655b7e3905b159d761e91c53c9"),
				Object:    MustIDFromString("3325fd8a973321fd59455492976c042dde3fd1ca"),
				Type:      "tag",
				Tagger:    parseAuthorLine(t, "Foo Bar <foo@bar.com> 1565789218 +0300"),
				Message:   "Add changelog of v1.9.1 (#7859)\n\n* add changelog of v1.9.1\n* Update CHANGELOG.md\n",
				Signature: nil,
			},
		},

		{
			name: "annotated tag with signature",

			givenRef: map[string]string{
				"objecttype":    "tag",
				"refname:short": "v0.0.1",
				"object":        "3325fd8a973321fd59455492976c042dde3fd1ca",
				"objectname":    "8c68a1f06fc59c655b7e3905b159d761e91c53c9",
				"creator":       "Foo Bar <foo@bar.com> 1565789218 +0300",
				"contents": `Add changelog of v1.9.1 (#7859)

* add changelog of v1.9.1
* Update CHANGELOG.md
-----BEGIN PGP SIGNATURE-----

aBCGzBAABCgAdFiEEyWRwv/q1Q6IjSv+D4IPOwzt33PoFAmI8jbIACgkQ4IPOwzt3
3PoRuAv9FVSbPBXvzECubls9KQd7urwEvcfG20Uf79iBwifQJUv+egNQojrs6APT
T4CdIXeGRpwJZaGTUX9RWnoDO1SLXAWnc82CypWraNwrHq8Go2YeoVu0Iy3vb0EU
REdob/tXYZecMuP8AjhUR0XfdYaERYAvJ2dYsH/UkFrqDjM3V4kPXWG+R5DCaZiE
slB5U01i4Dwb/zm/ckzhUGEcOgcnpOKX8SnY5kYRVDY47dl/yJZ1u2XWir3mu60G
1geIitH7StBddHi/8rz+sJwTfcVaLjn2p59p/Dr9aGbk17GIaKq1j0pZA2lKT0Xt
f9jDqU+9vCxnKgjSDhrwN69LF2jT47ZFjEMGV/wFPOa1EBxVWpgQ/CfEolBlbUqx
yVpbxi/6AOK2lmG130e9jEZJcu+WeZUeq851WgKSEkf2d5f/JpwtSTEOlOedu6V6
kl845zu5oE2nKM4zMQ7XrYQn538I31ps+VGQ0H8R07WrZP8WKUWugL2cU8KmXFwg
qbHDASXl
=2yGi
-----END PGP SIGNATURE-----

`,
				"contents:signature": `-----BEGIN PGP SIGNATURE-----

aBCGzBAABCgAdFiEEyWRwv/q1Q6IjSv+D4IPOwzt33PoFAmI8jbIACgkQ4IPOwzt3
3PoRuAv9FVSbPBXvzECubls9KQd7urwEvcfG20Uf79iBwifQJUv+egNQojrs6APT
T4CdIXeGRpwJZaGTUX9RWnoDO1SLXAWnc82CypWraNwrHq8Go2YeoVu0Iy3vb0EU
REdob/tXYZecMuP8AjhUR0XfdYaERYAvJ2dYsH/UkFrqDjM3V4kPXWG+R5DCaZiE
slB5U01i4Dwb/zm/ckzhUGEcOgcnpOKX8SnY5kYRVDY47dl/yJZ1u2XWir3mu60G
1geIitH7StBddHi/8rz+sJwTfcVaLjn2p59p/Dr9aGbk17GIaKq1j0pZA2lKT0Xt
f9jDqU+9vCxnKgjSDhrwN69LF2jT47ZFjEMGV/wFPOa1EBxVWpgQ/CfEolBlbUqx
yVpbxi/6AOK2lmG130e9jEZJcu+WeZUeq851WgKSEkf2d5f/JpwtSTEOlOedu6V6
kl845zu5oE2nKM4zMQ7XrYQn538I31ps+VGQ0H8R07WrZP8WKUWugL2cU8KmXFwg
qbHDASXl
=2yGi
-----END PGP SIGNATURE-----

`,
			},

			want: &Tag{
				Name:    "v0.0.1",
				ID:      MustIDFromString("8c68a1f06fc59c655b7e3905b159d761e91c53c9"),
				Object:  MustIDFromString("3325fd8a973321fd59455492976c042dde3fd1ca"),
				Type:    "tag",
				Tagger:  parseAuthorLine(t, "Foo Bar <foo@bar.com> 1565789218 +0300"),
				Message: "Add changelog of v1.9.1 (#7859)\n\n* add changelog of v1.9.1\n* Update CHANGELOG.md",
				Signature: &CommitGPGSignature{
					Signature: `-----BEGIN PGP SIGNATURE-----

aBCGzBAABCgAdFiEEyWRwv/q1Q6IjSv+D4IPOwzt33PoFAmI8jbIACgkQ4IPOwzt3
3PoRuAv9FVSbPBXvzECubls9KQd7urwEvcfG20Uf79iBwifQJUv+egNQojrs6APT
T4CdIXeGRpwJZaGTUX9RWnoDO1SLXAWnc82CypWraNwrHq8Go2YeoVu0Iy3vb0EU
REdob/tXYZecMuP8AjhUR0XfdYaERYAvJ2dYsH/UkFrqDjM3V4kPXWG+R5DCaZiE
slB5U01i4Dwb/zm/ckzhUGEcOgcnpOKX8SnY5kYRVDY47dl/yJZ1u2XWir3mu60G
1geIitH7StBddHi/8rz+sJwTfcVaLjn2p59p/Dr9aGbk17GIaKq1j0pZA2lKT0Xt
f9jDqU+9vCxnKgjSDhrwN69LF2jT47ZFjEMGV/wFPOa1EBxVWpgQ/CfEolBlbUqx
yVpbxi/6AOK2lmG130e9jEZJcu+WeZUeq851WgKSEkf2d5f/JpwtSTEOlOedu6V6
kl845zu5oE2nKM4zMQ7XrYQn538I31ps+VGQ0H8R07WrZP8WKUWugL2cU8KmXFwg
qbHDASXl
=2yGi
-----END PGP SIGNATURE-----

`,
					Payload: `object 3325fd8a973321fd59455492976c042dde3fd1ca
type commit
tag v0.0.1
tagger Foo Bar <foo@bar.com> 1565789218 +0300

Add changelog of v1.9.1 (#7859)

* add changelog of v1.9.1
* Update CHANGELOG.md
`,
				},
			},
		},
	}

	for _, test := range tests {
		tc := test // don't close over loop variable
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTagRef(tc.givenRef)

			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func parseAuthorLine(t *testing.T, committer string) *Signature {
	t.Helper()

	sig, err := newSignatureFromCommitline([]byte(committer))
	if err != nil {
		t.Fatalf("parse author line '%s': %v", committer, err)
	}

	return sig
}
