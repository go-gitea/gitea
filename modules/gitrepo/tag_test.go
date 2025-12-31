// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"code.gitea.io/gitea/modules/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetTagInfos(t *testing.T) {
	storage := &mockRepository{path: "repo1_bare"}

	tags, total, err := GetTagInfos(t.Context(), storage, 0, 0)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.Len(t, tags, 2)
	assert.Len(t, tags, total)
	assert.Equal(t, "signed-tag", tags[0].Name)
	assert.Equal(t, "36f97d9a96457e2bab511db30fe2db03893ebc64", tags[0].ID.String())
	assert.Equal(t, "tag", tags[0].Type)
	assert.Equal(t, "test", tags[1].Name)
	assert.Equal(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", tags[1].ID.String())
	assert.Equal(t, "tag", tags[1].Type)
}

func TestRepository_parseTagRef(t *testing.T) {
	tests := []struct {
		name string

		givenRef map[string]string

		want        *git.Tag
		wantErr     bool
		expectedErr error
	}{
		{
			name: "lightweight tag",

			givenRef: map[string]string{
				"objecttype":       "commit",
				"refname:lstrip=2": "v1.9.1",
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

			want: &git.Tag{
				Name:      "v1.9.1",
				ID:        git.MustIDFromString("ab23e4b7f4cd0caafe0174c0e7ef6d651ba72889"),
				Object:    git.MustIDFromString("ab23e4b7f4cd0caafe0174c0e7ef6d651ba72889"),
				Type:      "commit",
				Tagger:    git.ParseSignatureFromCommitLine("Foo Bar <foo@bar.com> 1565789218 +0300"),
				Message:   "Add changelog of v1.9.1 (#7859)\n\n* add changelog of v1.9.1\n* Update CHANGELOG.md\n",
				Signature: nil,
			},
		},

		{
			name: "annotated tag",

			givenRef: map[string]string{
				"objecttype":       "tag",
				"refname:lstrip=2": "v0.0.1",
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

			want: &git.Tag{
				Name:      "v0.0.1",
				ID:        git.MustIDFromString("8c68a1f06fc59c655b7e3905b159d761e91c53c9"),
				Object:    git.MustIDFromString("3325fd8a973321fd59455492976c042dde3fd1ca"),
				Type:      "tag",
				Tagger:    git.ParseSignatureFromCommitLine("Foo Bar <foo@bar.com> 1565789218 +0300"),
				Message:   "Add changelog of v1.9.1 (#7859)\n\n* add changelog of v1.9.1\n* Update CHANGELOG.md\n",
				Signature: nil,
			},
		},

		{
			name: "annotated tag with signature",

			givenRef: map[string]string{
				"objecttype":       "tag",
				"refname:lstrip=2": "v0.0.1",
				"object":           "3325fd8a973321fd59455492976c042dde3fd1ca",
				"objectname":       "8c68a1f06fc59c655b7e3905b159d761e91c53c9",
				"creator":          "Foo Bar <foo@bar.com> 1565789218 +0300",
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

			want: &git.Tag{
				Name:    "v0.0.1",
				ID:      git.MustIDFromString("8c68a1f06fc59c655b7e3905b159d761e91c53c9"),
				Object:  git.MustIDFromString("3325fd8a973321fd59455492976c042dde3fd1ca"),
				Type:    "tag",
				Tagger:  git.ParseSignatureFromCommitLine("Foo Bar <foo@bar.com> 1565789218 +0300"),
				Message: "Add changelog of v1.9.1 (#7859)\n\n* add changelog of v1.9.1\n* Update CHANGELOG.md",
				Signature: &git.CommitSignature{
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
