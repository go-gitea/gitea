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

func getCreateRepoFileOptions() *UpdateRepoFileOptions {
	return &UpdateRepoFileOptions{
		OldBranch:    "master",
		NewBranch:    "master",
		TreePath:     "new/file.txt",
		Message:      "Creates new/file.txt",
		Content:      "This is a NEW file",
		IsNewFile:    true,
		Author:       nil,
		Committer:    nil,
	}
}

func getUpdateRepoFileOptions() *UpdateRepoFileOptions {
	return &UpdateRepoFileOptions{
		OldBranch:    "master",
		NewBranch:    "master",
		TreePath:     "README.md",
		Message:      "Updates README.md",
		SHA:          "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Content:      "This is UPDATED content for the README file",
		IsNewFile:    false,
		Author:       nil,
		Committer:    nil,
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
			Verified: false,
			Reason: "",
			Signature: "",
			Payload: "",
		},
	}
}

func getExpectedFileResponseForUpdate(commitID string) *gitea.FileResponse {
	return &gitea.FileResponse{
		Content: &gitea.FileContentResponse{
			Name:        "README.md",
			Path:        "README.md",
			SHA:         "c7b380be487ac136e9ae8777e320fa09071a4fae",
			Size:        39,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/README.md",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/c7b380be487ac136e9ae8777e320fa09071a4fae",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/README.md",
			Type:        "blob",
			Links: &gitea.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/c7b380be487ac136e9ae8777e320fa09071a4fae",
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
			Verified: false,
			Reason: "",
			Signature: "",
			Payload: "",
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
	opts := getCreateRepoFileOptions()
	repo := ctx.Repo.Repository
	doer := ctx.User

	// Actual test
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(opts.NewBranch)
	expectedFileResponse := getExpectedFileResponseForCreate(commitID)
	assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
	// Can't get an exact match to the Commit Response since there are date stamps so we compare a few of the attributes
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
	opts := getUpdateRepoFileOptions()
	repo := ctx.Repo.Repository
	doer := ctx.User

	// Actual test
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(opts.NewBranch)
	expectedFileResponse := getExpectedFileResponseForUpdate(commitID)
	assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
	// Can't get an exact match to the Commit Response since there are date stamps so we compare a few of the attributes
	assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
	assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
	assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
	assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
}

// Test opts with branch names removed, same results
func TestUpdateRepoFileWithoutBranchNames(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	opts := getUpdateRepoFileOptions()
	opts.OldBranch = ""
	opts.NewBranch = ""
	repo := ctx.Repo.Repository
	doer := ctx.User

	// Test #1 - Update README.md file
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(opts.NewBranch)
	expectedFileContentResponse := getExpectedFileResponseForUpdate(commitID)
	assert.EqualValues(t, expectedFileContentResponse, fileResponse.Content)
}

func TestUpdateRepoFileErrors(t *testing.T) {
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

	// Test #1 - bad branch
	opts := getUpdateRepoFileOptions()
	opts.OldBranch = "bad_branch"
	fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Error(t, err)
	assert.Nil(t, fileResponse)
	expectedError := "branch does not exist [name: "+opts.OldBranch+"]"
	assert.EqualError(t, err, expectedError)

	// Test #2 - bad SHA
	opts = getUpdateRepoFileOptions()
	origSHA := opts.SHA
	opts.SHA = "bad_sha"
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file sha does not match ["+opts.SHA+" != "+origSHA+"]"
	assert.EqualError(t, err, expectedError)

	// Test #3 - new branch already exists
	opts = getUpdateRepoFileOptions()
	opts.NewBranch = "develop"
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "branch already exists [name: "+opts.NewBranch+"]"
	assert.EqualError(t, err, expectedError)

	// Test #4 - repo is nil
	opts = getUpdateRepoFileOptions()
	fileResponse, err = CreateOrUpdateRepoFile(nil, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "repo cannot be nil"
	assert.EqualError(t, err, expectedError)

	// Test #5 - doer is nil
	opts = getUpdateRepoFileOptions()
	fileResponse, err = CreateOrUpdateRepoFile(repo, nil, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "doer cannot be nil"
	assert.EqualError(t, err, expectedError)

	// Test #6 - opts is nil:
	opts = getUpdateRepoFileOptions()
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, nil)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "opts cannot be nil"
	assert.EqualError(t, err, expectedError)

	// Test #7 - treePath is empty:
	opts = getUpdateRepoFileOptions()
	opts.TreePath = ""
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file name is invalid: "
	assert.EqualError(t, err, expectedError)

	// Test #8 - treePath is a git directory:
	opts = getUpdateRepoFileOptions()
	opts.TreePath = ".git"
	fileResponse, err = CreateOrUpdateRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file name is invalid: "+opts.TreePath
	assert.EqualError(t, err, expectedError)
}
