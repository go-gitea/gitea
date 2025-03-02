// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitsCountSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare_sha256")

	commitsCount, err := CommitsCount(DefaultContext,
		CommitsCountOptions{
			RepoPath: bareRepo1Path,
			Revision: []string{"f004f41359117d319dedd0eaab8c5259ee2263da839dcba33637997458627fdc"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestCommitsCountWithoutBaseSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare_sha256")

	commitsCount, err := CommitsCount(DefaultContext,
		CommitsCountOptions{
			RepoPath: bareRepo1Path,
			Not:      "main",
			Revision: []string{"branch1"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), commitsCount)
}

func TestGetFullCommitIDSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare_sha256")

	id, err := GetFullCommitID(DefaultContext, bareRepo1Path, "f004f4")
	assert.NoError(t, err)
	assert.Equal(t, "f004f41359117d319dedd0eaab8c5259ee2263da839dcba33637997458627fdc", id)
}

func TestGetFullCommitIDErrorSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare_sha256")

	id, err := GetFullCommitID(DefaultContext, bareRepo1Path, "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}

func TestCommitFromReaderSha256(t *testing.T) {
	commitString := `9433b2a62b964c17a4485ae180f45f595d3e69d31b786087775e28c6b6399df0 commit 1114
tree e7f9e96dd79c09b078cac8b303a7d3b9d65ff9b734e86060a4d20409fd379f9e
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

signed commit`

	sha := &Sha256Hash{
		0x94, 0x33, 0xb2, 0xa6, 0x2b, 0x96, 0x4c, 0x17, 0xa4, 0x48, 0x5a, 0xe1, 0x80, 0xf4, 0x5f, 0x59,
		0x5d, 0x3e, 0x69, 0xd3, 0x1b, 0x78, 0x60, 0x87, 0x77, 0x5e, 0x28, 0xc6, 0xb6, 0x39, 0x9d, 0xf0,
	}
	gitRepo, err := openRepositoryWithDefaultContext(filepath.Join(testReposDir, "repo1_bare_sha256"))
	assert.NoError(t, err)
	assert.NotNil(t, gitRepo)
	defer gitRepo.Close()

	commitFromReader, err := CommitFromReader(gitRepo, sha, strings.NewReader(commitString))
	assert.NoError(t, err)
	require.NotNil(t, commitFromReader)
	assert.EqualValues(t, sha, commitFromReader.ID)
	assert.EqualValues(t, `-----BEGIN PGP SIGNATURE-----

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
`, commitFromReader.Signature.Signature)
	assert.EqualValues(t, `tree e7f9e96dd79c09b078cac8b303a7d3b9d65ff9b734e86060a4d20409fd379f9e
parent 26e9ccc29fad747e9c5d9f4c9ddeb7eff61cc45ef6a8dc258cbeb181afc055e8
author Adam Majer <amajer@suse.de> 1698676906 +0100
committer Adam Majer <amajer@suse.de> 1698676906 +0100

signed commit`, commitFromReader.Signature.Payload)
	assert.EqualValues(t, "Adam Majer <amajer@suse.de>", commitFromReader.Author.String())

	commitFromReader2, err := CommitFromReader(gitRepo, sha, strings.NewReader(commitString+"\n\n"))
	assert.NoError(t, err)
	commitFromReader.CommitMessage += "\n\n"
	commitFromReader.Signature.Payload += "\n\n"
	assert.EqualValues(t, commitFromReader, commitFromReader2)
}

func TestHasPreviousCommitSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare_sha256")

	repo, err := openRepositoryWithDefaultContext(bareRepo1Path)
	assert.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit("f004f41359117d319dedd0eaab8c5259ee2263da839dcba33637997458627fdc")
	assert.NoError(t, err)

	objectFormat, err := repo.GetObjectFormat()
	assert.NoError(t, err)

	parentSHA := MustIDFromString("b0ec7af4547047f12d5093e37ef8f1b3b5415ed8ee17894d43a34d7d34212e9c")
	notParentSHA := MustIDFromString("42e334efd04cd36eea6da0599913333c26116e1a537ca76e5b6e4af4dda00236")
	assert.Equal(t, objectFormat, parentSHA.Type())
	assert.Equal(t, "sha256", objectFormat.Name())

	haz, err := commit.HasPreviousCommit(parentSHA)
	assert.NoError(t, err)
	assert.True(t, haz)

	hazNot, err := commit.HasPreviousCommit(notParentSHA)
	assert.NoError(t, err)
	assert.False(t, hazNot)

	selfNot, err := commit.HasPreviousCommit(commit.ID)
	assert.NoError(t, err)
	assert.False(t, selfNot)
}

func TestGetCommitFileStatusMergesSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo6_merge_sha256")

	commitFileStatus, err := GetCommitFileStatus(DefaultContext, bareRepo1Path, "d2e5609f630dd8db500f5298d05d16def282412e3e66ed68cc7d0833b29129a1")
	assert.NoError(t, err)

	expected := CommitFileStatus{
		[]string{
			"add_file.txt",
		},
		[]string{},
		[]string{
			"to_modify.txt",
		},
	}

	assert.Equal(t, expected.Added, commitFileStatus.Added)
	assert.Equal(t, expected.Removed, commitFileStatus.Removed)
	assert.Equal(t, expected.Modified, commitFileStatus.Modified)

	expected = CommitFileStatus{
		[]string{},
		[]string{
			"to_remove.txt",
		},
		[]string{},
	}

	commitFileStatus, err = GetCommitFileStatus(DefaultContext, bareRepo1Path, "da1ded40dc8e5b7c564171f4bf2fc8370487decfb1cb6a99ef28f3ed73d09172")
	assert.NoError(t, err)

	assert.Equal(t, expected.Added, commitFileStatus.Added)
	assert.Equal(t, expected.Removed, commitFileStatus.Removed)
	assert.Equal(t, expected.Modified, commitFileStatus.Modified)
}
