// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitDiffTree(t *testing.T) {
	test := []struct {
		Name         string
		RepoPath     string
		BaseSha      string
		HeadSha      string
		useMergeBase bool
		Expected     *DiffTree
	}{
		{
			Name:     "happy path",
			RepoPath: "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:  "72866af952e98d02a73003501836074b286a78f6",
			HeadSha:  "d8e0bbb45f200e67d9a784ce55bd90821af45ebd",
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{
					{
						Status:     "modified",
						HeadPath:   "LICENSE",
						BasePath:   "LICENSE",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeBlob,
						HeadBlobID: "ee469963e76ae1bb7ee83d7510df2864e6c8c640",
						BaseBlobID: "c996f4725be8fc8c1d1c776e58c97ddc5d03b336",
					},
					{
						Status:     "modified",
						HeadPath:   "README.md",
						BasePath:   "README.md",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeBlob,
						HeadBlobID: "9dfc0a6257d8eff526f0cfaf6a8ea950f55a9dba",
						BaseBlobID: "074e590b8e64898b02beef03ece83f962c94f54c",
					},
				},
			},
		},
		{
			Name:     "first commit (no parent)",
			RepoPath: "../../modules/git/tests/repos/repo5_pulls",
			HeadSha:  "72866af952e98d02a73003501836074b286a78f6",
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{
					{
						Status:     "added",
						HeadPath:   ".gitignore",
						BasePath:   ".gitignore",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeNoEntry,
						HeadBlobID: "f1c181ec9c5c921245027c6b452ecfc1d3626364",
						BaseBlobID: "0000000000000000000000000000000000000000",
					},
					{
						Status:     "added",
						HeadPath:   "LICENSE",
						BasePath:   "LICENSE",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeNoEntry,
						HeadBlobID: "c996f4725be8fc8c1d1c776e58c97ddc5d03b336",
						BaseBlobID: "0000000000000000000000000000000000000000",
					},
					{
						Status:     "added",
						HeadPath:   "README.md",
						BasePath:   "README.md",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeNoEntry,
						HeadBlobID: "074e590b8e64898b02beef03ece83f962c94f54c",
						BaseBlobID: "0000000000000000000000000000000000000000",
					},
				},
			},
		},
		{
			Name:         "first commit (no parent), merge base = true",
			RepoPath:     "../../modules/git/tests/repos/repo5_pulls",
			HeadSha:      "72866af952e98d02a73003501836074b286a78f6",
			useMergeBase: true,
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{
					{
						Status:     "added",
						HeadPath:   ".gitignore",
						BasePath:   ".gitignore",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeNoEntry,
						HeadBlobID: "f1c181ec9c5c921245027c6b452ecfc1d3626364",
						BaseBlobID: "0000000000000000000000000000000000000000",
					},
					{
						Status:     "added",
						HeadPath:   "LICENSE",
						BasePath:   "LICENSE",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeNoEntry,
						HeadBlobID: "c996f4725be8fc8c1d1c776e58c97ddc5d03b336",
						BaseBlobID: "0000000000000000000000000000000000000000",
					},
					{
						Status:     "added",
						HeadPath:   "README.md",
						BasePath:   "README.md",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeNoEntry,
						HeadBlobID: "074e590b8e64898b02beef03ece83f962c94f54c",
						BaseBlobID: "0000000000000000000000000000000000000000",
					},
				},
			},
		},
		{
			Name:     "base and head same",
			RepoPath: "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:  "ed8f4d2fa5b2420706580d191f5dd50c4e491f3f",
			HeadSha:  "ed8f4d2fa5b2420706580d191f5dd50c4e491f3f",
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{},
			},
		},
		{
			Name:         "useMergeBase false",
			RepoPath:     "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:      "ed8f4d2fa5b2420706580d191f5dd50c4e491f3f",
			HeadSha:      "111cac04bd7d20301964e27a93698aabb5781b80", // this commit can be found on the update-readme branch
			useMergeBase: false,
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{
					{
						Status:     "modified",
						HeadPath:   "LICENSE",
						BasePath:   "LICENSE",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeBlob,
						HeadBlobID: "c996f4725be8fc8c1d1c776e58c97ddc5d03b336",
						BaseBlobID: "ed5119b3c1f45547b6785bc03eac7f87570fa17f",
					},

					{
						Status:     "modified",
						HeadPath:   "README.md",
						BasePath:   "README.md",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeBlob,
						HeadBlobID: "fb39771a8865c9a67f2ab9b616c854805664553c",
						BaseBlobID: "9dfc0a6257d8eff526f0cfaf6a8ea950f55a9dba",
					},
				},
			},
		},
		{
			Name:         "useMergeBase true",
			RepoPath:     "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:      "ed8f4d2fa5b2420706580d191f5dd50c4e491f3f",
			HeadSha:      "111cac04bd7d20301964e27a93698aabb5781b80", // this commit can be found on the update-readme branch
			useMergeBase: true,
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{
					{
						Status:     "modified",
						HeadPath:   "README.md",
						BasePath:   "README.md",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeBlob,
						HeadBlobID: "fb39771a8865c9a67f2ab9b616c854805664553c",
						BaseBlobID: "9dfc0a6257d8eff526f0cfaf6a8ea950f55a9dba",
					},
				},
			},
		},
		{
			Name:         "no base set",
			RepoPath:     "../../modules/git/tests/repos/repo5_pulls",
			HeadSha:      "d8e0bbb45f200e67d9a784ce55bd90821af45ebd", // this commit can be found on the update-readme branch
			useMergeBase: false,
			Expected: &DiffTree{
				Files: []*DiffTreeRecord{
					{
						Status:     "modified",
						HeadPath:   "LICENSE",
						BasePath:   "LICENSE",
						HeadMode:   git.EntryModeBlob,
						BaseMode:   git.EntryModeBlob,
						HeadBlobID: "ee469963e76ae1bb7ee83d7510df2864e6c8c640",
						BaseBlobID: "ed5119b3c1f45547b6785bc03eac7f87570fa17f",
					},
				},
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			gitRepo, err := git.OpenRepository(git.DefaultContext, tt.RepoPath)
			assert.NoError(t, err)
			defer gitRepo.Close()

			diffPaths, err := GetDiffTree(db.DefaultContext, gitRepo, tt.useMergeBase, tt.BaseSha, tt.HeadSha)
			require.NoError(t, err)

			assert.Equal(t, tt.Expected, diffPaths)
		})
	}
}

func TestParseGitDiffTree(t *testing.T) {
	test := []struct {
		Name      string
		GitOutput string
		Expected  []*DiffTreeRecord
	}{
		{
			Name:      "file change",
			GitOutput: ":100644 100644 64e43d23bcd08db12563a0a4d84309cadb437e1a 5dbc7792b5bb228647cfcc8dfe65fc649119dedc M\tResources/views/curriculum/edit.blade.php",
			Expected: []*DiffTreeRecord{
				{
					Status:     "modified",
					HeadPath:   "Resources/views/curriculum/edit.blade.php",
					BasePath:   "Resources/views/curriculum/edit.blade.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "5dbc7792b5bb228647cfcc8dfe65fc649119dedc",
					BaseBlobID: "64e43d23bcd08db12563a0a4d84309cadb437e1a",
				},
			},
		},
		{
			Name:      "file added",
			GitOutput: ":000000 100644 0000000000000000000000000000000000000000 0063162fb403db15ceb0517b34ab782e4e58b619 A\tResources/views/class/index.blade.php",
			Expected: []*DiffTreeRecord{
				{
					Status:     "added",
					HeadPath:   "Resources/views/class/index.blade.php",
					BasePath:   "Resources/views/class/index.blade.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeNoEntry,
					HeadBlobID: "0063162fb403db15ceb0517b34ab782e4e58b619",
					BaseBlobID: "0000000000000000000000000000000000000000",
				},
			},
		},
		{
			Name:      "file deleted",
			GitOutput: ":100644 000000 bac4286303c8c0017ea2f0a48c561ddcc0330a14 0000000000000000000000000000000000000000 D\tResources/views/classes/index.blade.php",
			Expected: []*DiffTreeRecord{
				{
					Status:     "deleted",
					HeadPath:   "Resources/views/classes/index.blade.php",
					BasePath:   "Resources/views/classes/index.blade.php",
					HeadMode:   git.EntryModeNoEntry,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "0000000000000000000000000000000000000000",
					BaseBlobID: "bac4286303c8c0017ea2f0a48c561ddcc0330a14",
				},
			},
		},
		{
			Name:      "file renamed",
			GitOutput: ":100644 100644 c8a055cfb45cd39747292983ad1797ceab40f5b1 97248f79a90aaf81fe7fd74b33c1cb182dd41783 R087\tDatabase/Seeders/AdminDatabaseSeeder.php\tDatabase/Seeders/AcademicDatabaseSeeder.php",
			Expected: []*DiffTreeRecord{
				{
					Status:     "renamed",
					Score:      87,
					HeadPath:   "Database/Seeders/AcademicDatabaseSeeder.php",
					BasePath:   "Database/Seeders/AdminDatabaseSeeder.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "97248f79a90aaf81fe7fd74b33c1cb182dd41783",
					BaseBlobID: "c8a055cfb45cd39747292983ad1797ceab40f5b1",
				},
			},
		},
		{
			Name:      "no changes",
			GitOutput: ``,
			Expected:  []*DiffTreeRecord{},
		},
		{
			Name: "multiple changes",
			GitOutput: ":000000 100644 0000000000000000000000000000000000000000 db736b44533a840981f1f17b7029d0f612b69550 A\tHttp/Controllers/ClassController.php\n" +
				":100644 000000 9a4d2344d4d0145db7c91b3f3e123c74367d4ef4 0000000000000000000000000000000000000000 D\tHttp/Controllers/ClassesController.php\n" +
				":100644 100644 f060d6aede65d423f49e7dc248dfa0d8835ef920 b82c8e39a3602dedadb44669956d6eb5b6a7cc86 M\tHttp/Controllers/ProgramDirectorController.php\n",
			Expected: []*DiffTreeRecord{
				{
					Status:     "added",
					HeadPath:   "Http/Controllers/ClassController.php",
					BasePath:   "Http/Controllers/ClassController.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeNoEntry,
					HeadBlobID: "db736b44533a840981f1f17b7029d0f612b69550",
					BaseBlobID: "0000000000000000000000000000000000000000",
				},
				{
					Status:     "deleted",
					HeadPath:   "Http/Controllers/ClassesController.php",
					BasePath:   "Http/Controllers/ClassesController.php",
					HeadMode:   git.EntryModeNoEntry,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "0000000000000000000000000000000000000000",
					BaseBlobID: "9a4d2344d4d0145db7c91b3f3e123c74367d4ef4",
				},
				{
					Status:     "modified",
					HeadPath:   "Http/Controllers/ProgramDirectorController.php",
					BasePath:   "Http/Controllers/ProgramDirectorController.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "b82c8e39a3602dedadb44669956d6eb5b6a7cc86",
					BaseBlobID: "f060d6aede65d423f49e7dc248dfa0d8835ef920",
				},
			},
		},
		{
			Name: "spaces in file path",
			GitOutput: ":000000 100644 0000000000000000000000000000000000000000 db736b44533a840981f1f17b7029d0f612b69550 A\tHttp /Controllers/Class Controller.php\n" +
				":100644 000000 9a4d2344d4d0145db7c91b3f3e123c74367d4ef4 0000000000000000000000000000000000000000 D\tHttp/Cont rollers/Classes Controller.php\n" +
				":100644 100644 f060d6aede65d423f49e7dc248dfa0d8835ef920 b82c8e39a3602dedadb44669956d6eb5b6a7cc86 R010\tHttp/Controllers/Program Director Controller.php\tHttp/Cont rollers/ProgramDirectorController.php\n",
			Expected: []*DiffTreeRecord{
				{
					Status:     "added",
					HeadPath:   "Http /Controllers/Class Controller.php",
					BasePath:   "Http /Controllers/Class Controller.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeNoEntry,
					HeadBlobID: "db736b44533a840981f1f17b7029d0f612b69550",
					BaseBlobID: "0000000000000000000000000000000000000000",
				},
				{
					Status:     "deleted",
					HeadPath:   "Http/Cont rollers/Classes Controller.php",
					BasePath:   "Http/Cont rollers/Classes Controller.php",
					HeadMode:   git.EntryModeNoEntry,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "0000000000000000000000000000000000000000",
					BaseBlobID: "9a4d2344d4d0145db7c91b3f3e123c74367d4ef4",
				},
				{
					Status:     "renamed",
					Score:      10,
					HeadPath:   "Http/Cont rollers/ProgramDirectorController.php",
					BasePath:   "Http/Controllers/Program Director Controller.php",
					HeadMode:   git.EntryModeBlob,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "b82c8e39a3602dedadb44669956d6eb5b6a7cc86",
					BaseBlobID: "f060d6aede65d423f49e7dc248dfa0d8835ef920",
				},
			},
		},
		{
			Name:      "file type changed",
			GitOutput: ":100644 120000 344e0ca8aa791cc4164fb0ea645f334fd40d00f0 a7c2973de00bfdc6ca51d315f401b5199fe01dc3 T\twebpack.mix.js",
			Expected: []*DiffTreeRecord{
				{
					Status:     "typechanged",
					HeadPath:   "webpack.mix.js",
					BasePath:   "webpack.mix.js",
					HeadMode:   git.EntryModeSymlink,
					BaseMode:   git.EntryModeBlob,
					HeadBlobID: "a7c2973de00bfdc6ca51d315f401b5199fe01dc3",
					BaseBlobID: "344e0ca8aa791cc4164fb0ea645f334fd40d00f0",
				},
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			entries, err := parseGitDiffTree(strings.NewReader(tt.GitOutput))
			assert.NoError(t, err)
			assert.Equal(t, tt.Expected, entries)
		})
	}
}

func TestGitDiffTreeErrors(t *testing.T) {
	test := []struct {
		Name     string
		RepoPath string
		BaseSha  string
		HeadSha  string
	}{
		{
			Name:     "head doesn't exist",
			RepoPath: "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:  "f32b0a9dfd09a60f616f29158f772cedd89942d2",
			HeadSha:  "asdfasdfasdf",
		},
		{
			Name:     "base doesn't exist",
			RepoPath: "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:  "asdfasdfasdf",
			HeadSha:  "f32b0a9dfd09a60f616f29158f772cedd89942d2",
		},
		{
			Name:     "head not set",
			RepoPath: "../../modules/git/tests/repos/repo5_pulls",
			BaseSha:  "f32b0a9dfd09a60f616f29158f772cedd89942d2",
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			gitRepo, err := git.OpenRepository(git.DefaultContext, tt.RepoPath)
			assert.NoError(t, err)
			defer gitRepo.Close()

			diffPaths, err := GetDiffTree(db.DefaultContext, gitRepo, true, tt.BaseSha, tt.HeadSha)
			assert.Error(t, err)
			assert.Nil(t, diffPaths)
		})
	}
}
