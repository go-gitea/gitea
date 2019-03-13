// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file_handling

import (
	"code.gitea.io/git"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/sdk/gitea"
)

func getCreateRepoFileOptions(repo *models.Repository) *UpdateRepoFileOptions {
	return &UpdateRepoFileOptions{
		OldBranch: repo.DefaultBranch,
		NewBranch: repo.DefaultBranch,
		TreePath:  "new/file.txt",
		Message:   "Creates new/file.txt",
		Content:   "This is a NEW file",
		IsNewFile: true,
		Author:    nil,
		Committer: nil,
	}
}

func getUpdateRepoFileOptions(repo *models.Repository) *UpdateRepoFileOptions {
	return &UpdateRepoFileOptions{
		OldBranch: repo.DefaultBranch,
		NewBranch: repo.DefaultBranch,
		TreePath:  "README.md",
		Message:   "Updates README.md",
		SHA:       "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Content:   "This is UPDATED content for the README file",
		IsNewFile: false,
		Author:    nil,
		Committer: nil,
	}
}

func getExpectedFileResponseForCreate(commitID string) *gitea.FileResponse {
	return &gitea.FileResponse{
		Content: &gitea.FileContentResponse{
			Name:        "file.txt",
			Path:        "new/file.txt",
			SHA:         "103ff9234cefeee5ec5361d22b49fbb04d385885",
			Size:        18,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/new/file.txt",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/new/file.txt",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/103ff9234cefeee5ec5361d22b49fbb04d385885",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/new/file.txt",
			Type:        "blob",
			Links: &gitea.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/new/file.txt",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/103ff9234cefeee5ec5361d22b49fbb04d385885",
				HTMLURL: "https://try.gitea.io/user2/repo1/blob/master/new/file.txt",
			},
		},
		Commit: &gitea.FileCommitResponse{
			CommitMeta: &gitea.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: "https://try.gitea.io/user2/repo1/commit/" + commitID,
			Author: &gitea.CommitUser{
				Name:  "User Two",
				Email: "user2@",
				Date:  time.Now().UTC().Format(time.RFC3339),
			},
			Committer: &gitea.CommitUser{
				Name:  "User Two",
				Email: "user2@",
				Date:  time.Now().UTC().Format(time.RFC3339),
			},
			Parents: []*gitea.CommitMeta{
				{
					URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
					SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
				},
			},
			Message: "Updates README.md\n",
			Tree: &gitea.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
				SHA: "f93e3a1a1525fb5b91020git dda86e44810c87a2d7bc",
			},
		},
		Verification: &gitea.PayloadCommitVerification{
			Verified:  false,
			Reason:    "unsigned",
			Signature: "",
			Payload:   "",
		},
	}
}

func getExpectedFileResponseForUpdate(commitID string) *gitea.FileResponse {
	return &gitea.FileResponse{
		Content: &gitea.FileContentResponse{
			Name:        "README.md",
			Path:        "README.md",
			SHA:         "dbf8d00e022e05b7e5cf7e535de857de57925647",
			Size:        43,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/README.md",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/dbf8d00e022e05b7e5cf7e535de857de57925647",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/README.md",
			Type:        "blob",
			Links: &gitea.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/dbf8d00e022e05b7e5cf7e535de857de57925647",
				HTMLURL: "https://try.gitea.io/user2/repo1/blob/master/README.md",
			},
		},
		Commit: &gitea.FileCommitResponse{
			CommitMeta: &gitea.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: "https://try.gitea.io/user2/repo1/commit/" + commitID,
			Author: &gitea.CommitUser{
				Name:  "User Two",
				Email: "user2@",
				Date:  time.Now().UTC().Format(time.RFC3339),
			},
			Committer: &gitea.CommitUser{
				Name:  "User Two",
				Email: "user2@",
				Date:  time.Now().UTC().Format(time.RFC3339),
			},
			Parents: []*gitea.CommitMeta{
				{
					URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
					SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
				},
			},
			Message: "Updates README.md\n",
			Tree: &gitea.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
				SHA: "f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
			},
		},
		Verification: &gitea.PayloadCommitVerification{
			Verified:  false,
			Reason:    "unsigned",
			Signature: "",
			Payload:   "",
		},
	}
}

func TestCreateOrUpdateRepoFileForCreate(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getCreateRepoFileOptions(repo)

	// test
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(opts.NewBranch)
	expectedFileResponse := getExpectedFileResponseForCreate(commitID)
	assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
	assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
	assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
	assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
	assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
}

func TestCreateOrUpdateRepoFileForUpdate(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getUpdateRepoFileOptions(repo)

	// test
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(opts.NewBranch)
	expectedFileResponse := getExpectedFileResponseForUpdate(commitID)
	assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
	assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
	assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
	assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
	assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
}

func TestCreateOrUpdateRepoFileForUpdateWithFileMove(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getUpdateRepoFileOptions(repo)
	suffix := "_new"
	opts.FromTreePath = "README.md"
	opts.TreePath = "README.md" + suffix // new file name, README.md_new

	// test
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commit, _ := gitRepo.GetBranchCommit(opts.NewBranch)
	expectedFileResponse := getExpectedFileResponseForUpdate(commit.ID.String())
	// assert that the old file no longer exists in the last commit of the branch
	fromEntry, err := commit.GetTreeEntryByPath(opts.FromTreePath)
	toEntry, err := commit.GetTreeEntryByPath(opts.TreePath)
	assert.Nil(t, fromEntry)  // Should no longer exist here
	assert.NotNil(t, toEntry) // Should exist here
	// assert SHA has remained the same but paths use the new file name
	assert.EqualValues(t, expectedFileResponse.Content.SHA, fileResponse.Content.SHA)
	assert.EqualValues(t, expectedFileResponse.Content.Name+suffix, fileResponse.Content.Name)
	assert.EqualValues(t, expectedFileResponse.Content.Path+suffix, fileResponse.Content.Path)
	assert.EqualValues(t, expectedFileResponse.Content.URL+suffix, fileResponse.Content.URL)
	assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
	assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
}

// Test opts with branch names removed, should get same results as above test
func TestCreateOrUpdateRepoFileWithoutBranchNames(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getUpdateRepoFileOptions(repo)
	opts.OldBranch = ""
	opts.NewBranch = ""

	// test
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(repo.DefaultBranch)
	expectedFileResponse := getExpectedFileResponseForUpdate(commitID)
	assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
}

func TestCreateOrUpdateRepoFileErrors(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User

	// test #1 - bad branch
	opts := getUpdateRepoFileOptions(repo)
	opts.OldBranch = "bad_branch"
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Error(t, err)
	assert.Nil(t, fileResponse)
	expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
	assert.EqualError(t, err, expectedError)

	// test #2 - bad SHA
	opts = getUpdateRepoFileOptions(repo)
	origSHA := opts.SHA
	opts.SHA = "bad_sha"
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file sha does not match [given: " + opts.SHA + ", expected: " + origSHA + "]"
	assert.EqualError(t, err, expectedError)

	// test #3 - new branch already exists
	opts = getUpdateRepoFileOptions(repo)
	opts.NewBranch = "develop"
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "branch already exists [name: " + opts.NewBranch + "]"
	assert.EqualError(t, err, expectedError)

	// test #4 - repo is nil
	opts = getUpdateRepoFileOptions(repo)
	fileResponse, err = CreateOrUpdateRepoFile(nil, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "repo cannot be nil"
	assert.EqualError(t, err, expectedError)

	// test #5 - doer is nil
	opts = getUpdateRepoFileOptions(repo)
	fileResponse, err = CreateOrUpdateRepoFile(repo, nil, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "doer cannot be nil"
	assert.EqualError(t, err, expectedError)

	// test #6 - opts is nil:
	opts = getUpdateRepoFileOptions(repo)
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, nil)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "opts cannot be nil"
	assert.EqualError(t, err, expectedError)

	// test #7 - treePath is empty:
	opts = getUpdateRepoFileOptions(repo)
	opts.TreePath = ""
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file name is invalid: "
	assert.EqualError(t, err, expectedError)

	// test #8 - treePath is a git directory:
	opts = getUpdateRepoFileOptions(repo)
	opts.TreePath = ".git"
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file name is invalid: " + opts.TreePath
	assert.EqualError(t, err, expectedError)

	// test #9 - create file that already exists
	opts = getCreateRepoFileOptions(repo)
	opts.TreePath = "README.md" //already exists
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "repository file already exists [file_name: " + opts.TreePath + "]"
	assert.EqualError(t, err, expectedError)
}
