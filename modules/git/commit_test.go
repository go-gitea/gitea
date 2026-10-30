// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFullCommitID(t *testing.T) {
	id, err := GetFullCommitID(t.Context(), mockRepository("repo1_bare"), "8006ff9a")
	assert.NoError(t, err)
	assert.Equal(t, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	id, err := GetFullCommitID(t.Context(), mockRepository("repo1_bare"), "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}

func TestCommitFromReader(t *testing.T) {
	for _, testCase := range []struct {
		name, repo                               string
		sha                                      ObjectID
		commitString, signature, payload, author string
	}{
		{
			"TestCommitFromReader", "repo1_bare",
			&Sha1Hash{0xfe, 0xaf, 0x4b, 0xa6, 0xbc, 0x63, 0x5f, 0xec, 0x44, 0x2f, 0x46, 0xdd, 0xd4, 0x51, 0x24, 0x16, 0xec, 0x43, 0xc2, 0xc2},
			`tree f1a6cb52b2d16773290cefe49ad0684b50a4f930
parent 37991dec2c8e592043f47155ce4808d4580f9123
author silverwind <me@silverwind.io> 1563741793 +0200
committer silverwind <me@silverwind.io> 1563741793 +0200
gpgsig -----BEGIN PGP SIGNATURE-----
` + " " + `
 iQIzBAABCAAdFiEEWPb2jX6FS2mqyJRQLmK0HJOGlEMFAl00zmEACgkQLmK0HJOG
 lEMDFBAAhQKKqLD1VICygJMEB8t1gBmNLgvziOLfpX4KPWdPtBk3v/QJ7OrfMrVK
 xlC4ZZyx6yMm1Q7GzmuWykmZQJ9HMaHJ49KAbh5MMjjV/+OoQw9coIdo8nagRUld
 vX8QHzNZ6Agx77xHuDJZgdHKpQK3TrMDsxzoYYMvlqoLJIDXE1Sp7KYNy12nhdRg
 R6NXNmW8oMZuxglkmUwayMiPS+N4zNYqv0CXYzlEqCOgq9MJUcAMHt+KpiST+sm6
 FWkJ9D+biNPyQ9QKf1AE4BdZia4lHfPYU/C/DEL/a5xQuuop/zMQZoGaIA4p2zGQ
 /maqYxEIM/yRBQpT1jlODKPJrMEgx7SgY2hRU47YZ4fj6350fb6fNBtiiMAfJbjL
 S3Gh85E9fm3hJaNSPKAaJFYL1Ya2svuWfgHj677C56UcmYis7fhiiy1aJuYdHnSm
 sD53z/f0J+We4VZjY+pidvA9BGZPFVdR3wd3xGs8/oH6UWaLJAMGkLG6dDb3qDLm
 1LFZwsX8sdD32i1SiWanYQYSYMyFWr0awi4xdoMtYCL7uKBYtwtPyvq3cj4IrJlb
 mfeFhT57UbE4qukTDIQ0Y0WM40UYRTakRaDY7ubhXgLgx09Cnp9XTVMsHgT6j9/i
 1pxsB104XLWjQHTjr1JtiaBQEwFh9r2OKTcpvaLcbNtYpo7CzOs=
 =FRsO
 -----END PGP SIGNATURE-----

empty commit`,
			`-----BEGIN PGP SIGNATURE-----

iQIzBAABCAAdFiEEWPb2jX6FS2mqyJRQLmK0HJOGlEMFAl00zmEACgkQLmK0HJOG
lEMDFBAAhQKKqLD1VICygJMEB8t1gBmNLgvziOLfpX4KPWdPtBk3v/QJ7OrfMrVK
xlC4ZZyx6yMm1Q7GzmuWykmZQJ9HMaHJ49KAbh5MMjjV/+OoQw9coIdo8nagRUld
vX8QHzNZ6Agx77xHuDJZgdHKpQK3TrMDsxzoYYMvlqoLJIDXE1Sp7KYNy12nhdRg
R6NXNmW8oMZuxglkmUwayMiPS+N4zNYqv0CXYzlEqCOgq9MJUcAMHt+KpiST+sm6
FWkJ9D+biNPyQ9QKf1AE4BdZia4lHfPYU/C/DEL/a5xQuuop/zMQZoGaIA4p2zGQ
/maqYxEIM/yRBQpT1jlODKPJrMEgx7SgY2hRU47YZ4fj6350fb6fNBtiiMAfJbjL
S3Gh85E9fm3hJaNSPKAaJFYL1Ya2svuWfgHj677C56UcmYis7fhiiy1aJuYdHnSm
sD53z/f0J+We4VZjY+pidvA9BGZPFVdR3wd3xGs8/oH6UWaLJAMGkLG6dDb3qDLm
1LFZwsX8sdD32i1SiWanYQYSYMyFWr0awi4xdoMtYCL7uKBYtwtPyvq3cj4IrJlb
mfeFhT57UbE4qukTDIQ0Y0WM40UYRTakRaDY7ubhXgLgx09Cnp9XTVMsHgT6j9/i
1pxsB104XLWjQHTjr1JtiaBQEwFh9r2OKTcpvaLcbNtYpo7CzOs=
=FRsO
-----END PGP SIGNATURE-----`,
			`tree f1a6cb52b2d16773290cefe49ad0684b50a4f930
parent 37991dec2c8e592043f47155ce4808d4580f9123
author silverwind <me@silverwind.io> 1563741793 +0200
committer silverwind <me@silverwind.io> 1563741793 +0200

empty commit`,
			"silverwind <me@silverwind.io>",
		},
		{
			"TestCommitWithEncodingFromReader", "repo1_bare",
			&Sha1Hash{0xfe, 0xaf, 0x4b, 0xa6, 0xbc, 0x63, 0x5f, 0xec, 0x44, 0x2f, 0x46, 0xdd, 0xd4, 0x51, 0x24, 0x16, 0xec, 0x43, 0xc2, 0xc2},
			strings.ReplaceAll(`tree ca3fad42080dd1a6d291b75acdfc46e5b9b307e5
parent 47b24e7ab977ed31c5a39989d570847d6d0052af
author KN4CK3R <admin@oldschoolhack.me> 1711702962 +0100
committer KN4CK3R <admin@oldschoolhack.me> 1711702962 +0100
encoding ISO-8859-1
gpgsig -----BEGIN PGP SIGNATURE-----
<SPACE>
 iQGzBAABCgAdFiEE9HRrbqvYxPT8PXbefPSEkrowAa8FAmYGg7IACgkQfPSEkrow
 Aa9olwv+P0HhtCM6CRvlUmPaqswRsDPNR4i66xyXGiSxdI9V5oJL7HLiQIM7KrFR
 gizKa2COiGtugv8fE+TKqXKaJx6uJUJEjaBd8E9Af9PrAzjWj+A84lU6/PgPS8hq
 zOfZraLOEWRH4tZcS+u2yFLu3ez2Wqh1xW5LNy7xqEedMXEFD1HwSJ0+pjacNkzr
 frp6Asyt7xRI6YmgFJZJoRsS3Ktr6rtKeRL2IErSQQyorOqj6gKrglhrhfG/114j
 FKB1v4or0WZ1DE8iP2SJZ3n+/K1IuWAINh7MVdb7PndfBPEa+IL+ucNk5uzEE8Jd
 G8smGxXUeFEt2cP1dj2W8EgAxuA9sTnH9dqI5aRqy5ifDjuya7Emm8sdOUvtGdmn
 SONRzusmu5n3DgV956REL7x62h7JuqmBz/12HZkr0z0zgXkcZ04q08pSJATX5N1F
 yN+tWxTsWg+zhDk96d5Esdo9JMjcFvPv0eioo30GAERaz1hoD7zCMT4jgUFTQwgz
 jw4YcO5u
 =r3UU
 -----END PGP SIGNATURE-----

ISO-8859-1`, "<SPACE>", " "),
			`-----BEGIN PGP SIGNATURE-----

iQGzBAABCgAdFiEE9HRrbqvYxPT8PXbefPSEkrowAa8FAmYGg7IACgkQfPSEkrow
Aa9olwv+P0HhtCM6CRvlUmPaqswRsDPNR4i66xyXGiSxdI9V5oJL7HLiQIM7KrFR
gizKa2COiGtugv8fE+TKqXKaJx6uJUJEjaBd8E9Af9PrAzjWj+A84lU6/PgPS8hq
zOfZraLOEWRH4tZcS+u2yFLu3ez2Wqh1xW5LNy7xqEedMXEFD1HwSJ0+pjacNkzr
frp6Asyt7xRI6YmgFJZJoRsS3Ktr6rtKeRL2IErSQQyorOqj6gKrglhrhfG/114j
FKB1v4or0WZ1DE8iP2SJZ3n+/K1IuWAINh7MVdb7PndfBPEa+IL+ucNk5uzEE8Jd
G8smGxXUeFEt2cP1dj2W8EgAxuA9sTnH9dqI5aRqy5ifDjuya7Emm8sdOUvtGdmn
SONRzusmu5n3DgV956REL7x62h7JuqmBz/12HZkr0z0zgXkcZ04q08pSJATX5N1F
yN+tWxTsWg+zhDk96d5Esdo9JMjcFvPv0eioo30GAERaz1hoD7zCMT4jgUFTQwgz
jw4YcO5u
=r3UU
-----END PGP SIGNATURE-----`,
			`tree ca3fad42080dd1a6d291b75acdfc46e5b9b307e5
parent 47b24e7ab977ed31c5a39989d570847d6d0052af
author KN4CK3R <admin@oldschoolhack.me> 1711702962 +0100
committer KN4CK3R <admin@oldschoolhack.me> 1711702962 +0100
encoding ISO-8859-1

ISO-8859-1`,
			"KN4CK3R <admin@oldschoolhack.me>",
		},
		{
			"TestCommitFromReaderSha256", "repo1_bare_sha256",
			&Sha256Hash{
				0x94, 0x33, 0xb2, 0xa6, 0x2b, 0x96, 0x4c, 0x17, 0xa4, 0x48, 0x5a, 0xe1, 0x80, 0xf4, 0x5f, 0x59,
				0x5d, 0x3e, 0x69, 0xd3, 0x1b, 0x78, 0x60, 0x87, 0x77, 0x5e, 0x28, 0xc6, 0xb6, 0x39, 0x9d, 0xf0,
			},
			`tree e7f9e96dd79c09b078cac8b303a7d3b9d65ff9b734e86060a4d20409fd379f9e
parent 26e9ccc29fad747e9c5d9f4c9ddeb7eff61cc45ef6a8dc258cbeb181afc055e8
author Adam Majer <amajer@suse.de> 1698676906 +0100
committer Adam Majer <amajer@suse.de> 1698676906 +0100
gpgsig-sha256 -----BEGIN PGP SIGNATURE-----
` + " " + `
 iQIrBAABCgAtFiEES+fB08xlgTrzSdQvhkUIsBsmec8FAmU/wKoPHGFtYWplckBz
 dXNlLmRlAAoJEIZFCLAbJnnP4s4PQIJATa++WPzR6/H4etT7bsOGoMyguEJYyWOd
 aTybplzT7QAL7h2to0QszGabtzMJPIA39xSFZNYNN30voK5YyyYibXluPKgjemfK
 WNXwF+gkwgZI38gSvKf+vlqI+EYyIFe19wOhiju0m8SIlB5NEPiWHa17q2mqmqqx
 1FWa2JdqLPYjAtSLFXeSZegrY5V1FxdemyMUONkg8YO9OSIMZiE0GsnnOXQ3xcT4
 JTCnmlUxIKw689UiEY80JopUIq+Wl7+qq9507IYYSUCyB6JazL42AKMzVCbD+qBP
 oOzh/hafYgk9H9qCQXaLbmvs17zXRpicig1bAzqgAy1FDelvpERyRTydEajSLIG6
 U1cRCkgXCZ0NfsYNPPmBa8b3+rnstypXYTbyMwTln7FfUAaGo6o9JYiPMkzxlmsy
 zfp/tcaY8+LlBL9aOJjtv+a0p+HrpCGd6CCa4ARfphTLq8QRSSh8uzlB9N+6HnRI
 VAEUo6ecdDxSpyt2naeg9pKus/BRi7P6g4B1hkk/zZstUX/QP4IQuAJbXjkvsC+X
 HKRr3NlRM/DygzTyj0gN74uoa0goCIbyAQhiT42nm0cuhM7uN/W0ayrlZjGF1cbR
 8NCJUL2Nwj0ywKIavC99Ipkb8AsFwpVT6U6effs6
 =xybZ
 -----END PGP SIGNATURE-----

signed commit`,
			`-----BEGIN PGP SIGNATURE-----

iQIrBAABCgAtFiEES+fB08xlgTrzSdQvhkUIsBsmec8FAmU/wKoPHGFtYWplckBz
dXNlLmRlAAoJEIZFCLAbJnnP4s4PQIJATa++WPzR6/H4etT7bsOGoMyguEJYyWOd
aTybplzT7QAL7h2to0QszGabtzMJPIA39xSFZNYNN30voK5YyyYibXluPKgjemfK
WNXwF+gkwgZI38gSvKf+vlqI+EYyIFe19wOhiju0m8SIlB5NEPiWHa17q2mqmqqx
1FWa2JdqLPYjAtSLFXeSZegrY5V1FxdemyMUONkg8YO9OSIMZiE0GsnnOXQ3xcT4
JTCnmlUxIKw689UiEY80JopUIq+Wl7+qq9507IYYSUCyB6JazL42AKMzVCbD+qBP
oOzh/hafYgk9H9qCQXaLbmvs17zXRpicig1bAzqgAy1FDelvpERyRTydEajSLIG6
U1cRCkgXCZ0NfsYNPPmBa8b3+rnstypXYTbyMwTln7FfUAaGo6o9JYiPMkzxlmsy
zfp/tcaY8+LlBL9aOJjtv+a0p+HrpCGd6CCa4ARfphTLq8QRSSh8uzlB9N+6HnRI
VAEUo6ecdDxSpyt2naeg9pKus/BRi7P6g4B1hkk/zZstUX/QP4IQuAJbXjkvsC+X
HKRr3NlRM/DygzTyj0gN74uoa0goCIbyAQhiT42nm0cuhM7uN/W0ayrlZjGF1cbR
8NCJUL2Nwj0ywKIavC99Ipkb8AsFwpVT6U6effs6
=xybZ
-----END PGP SIGNATURE-----`,
			`tree e7f9e96dd79c09b078cac8b303a7d3b9d65ff9b734e86060a4d20409fd379f9e
parent 26e9ccc29fad747e9c5d9f4c9ddeb7eff61cc45ef6a8dc258cbeb181afc055e8
author Adam Majer <amajer@suse.de> 1698676906 +0100
committer Adam Majer <amajer@suse.de> 1698676906 +0100

signed commit`,
			"Adam Majer <amajer@suse.de>",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			gitRepo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, testCase.repo))
			assert.NoError(t, err)
			assert.NotNil(t, gitRepo)
			defer gitRepo.Close()

			commitFromReader, err := CommitFromReader(testCase.sha, strings.NewReader(testCase.commitString))
			assert.NoError(t, err)
			require.NotNil(t, commitFromReader)
			assert.Equal(t, testCase.sha, commitFromReader.ID)
			assert.Equal(t, testCase.signature, commitFromReader.Signature.Signature)
			assert.Equal(t, testCase.payload, commitFromReader.Signature.Payload)
			assert.Equal(t, testCase.author, commitFromReader.Author.String())

			commitFromReader2, err := CommitFromReader(testCase.sha, strings.NewReader(testCase.commitString+"\n\n"))
			assert.NoError(t, err)
			commitFromReader.CommitMessage.MessageRaw += "\n\n"
			commitFromReader.Signature.Payload += "\n\n"
			assert.Equal(t, commitFromReader, commitFromReader2)
		})
	}
}

func TestHasPreviousCommit(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	repo, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit(t.Context(), "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0")
	assert.NoError(t, err)

	parentSHA := MustIDFromString("8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2")
	notParentSHA := MustIDFromString("2839944139e0de9737a044f78b0e4b40d989a9e3")

	haz, err := commit.HasPreviousCommit(t.Context(), repo, parentSHA)
	assert.NoError(t, err)
	assert.True(t, haz)

	hazNot, err := commit.HasPreviousCommit(t.Context(), repo, notParentSHA)
	assert.NoError(t, err)
	assert.False(t, hazNot)

	selfNot, err := commit.HasPreviousCommit(t.Context(), repo, commit.ID)
	assert.NoError(t, err)
	assert.False(t, selfNot)
}

func Test_GetCommitBranchStart(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer repo.Close()
	commit, err := repo.GetBranchCommit(t.Context(), "branch1")
	assert.NoError(t, err)
	assert.Equal(t, "2839944139e0de9737a044f78b0e4b40d989a9e3", commit.ID.String())

	startCommitID, err := repo.GetCommitBranchStart(t.Context(), os.Environ(), "branch1", commit.ID.String())
	assert.NoError(t, err)
	assert.NotEmpty(t, startCommitID)
	assert.Equal(t, "95bb4d39648ee7e325106df01a621c530863a653", startCommitID)
}
