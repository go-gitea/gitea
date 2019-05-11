// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
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

func getExpectedFileResponseForCreate(commitID string) *api.FileResponse {
	return &api.FileResponse{
		Content: &api.FileContentResponse{
			Name:        "file.txt",
			Path:        "new/file.txt",
			SHA:         "103ff9234cefeee5ec5361d22b49fbb04d385885",
			Size:        18,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/new/file.txt",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/new/file.txt",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/103ff9234cefeee5ec5361d22b49fbb04d385885",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/new/file.txt",
			Type:        "blob",
			Links: &api.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/new/file.txt",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/103ff9234cefeee5ec5361d22b49fbb04d385885",
				HTMLURL: "https://try.gitea.io/user2/repo1/blob/master/new/file.txt",
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: "https://try.gitea.io/user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Parents: []*api.CommitMeta{
				{
					URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
					SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
				},
			},
			Message: "Updates README.md\n",
			Tree: &api.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
				SHA: "f93e3a1a1525fb5b91020git dda86e44810c87a2d7bc",
			},
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "unsigned",
			Signature: "",
			Payload:   "",
		},
	}
}

func getExpectedFileResponseForUpdate(commitID string) *api.FileResponse {
	return &api.FileResponse{
		Content: &api.FileContentResponse{
			Name:        "README.md",
			Path:        "README.md",
			SHA:         "dbf8d00e022e05b7e5cf7e535de857de57925647",
			Size:        43,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/README.md",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/dbf8d00e022e05b7e5cf7e535de857de57925647",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/README.md",
			Type:        "blob",
			Links: &api.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/dbf8d00e022e05b7e5cf7e535de857de57925647",
				HTMLURL: "https://try.gitea.io/user2/repo1/blob/master/README.md",
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: "https://try.gitea.io/user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Parents: []*api.CommitMeta{
				{
					URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
					SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
				},
			},
			Message: "Updates README.md\n",
			Tree: &api.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
				SHA: "f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
			},
		},
		Verification: &api.PayloadCommitVerification{
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

	t.Run("bad branch", func(t *testing.T) {
		opts := getUpdateRepoFileOptions(repo)
		opts.OldBranch = "bad_branch"
		fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
		assert.Error(t, err)
		assert.Nil(t, fileResponse)
		expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("bad SHA", func(t *testing.T) {
		opts := getUpdateRepoFileOptions(repo)
		origSHA := opts.SHA
		opts.SHA = "bad_sha"
		fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "sha does not match [given: " + opts.SHA + ", expected: " + origSHA + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("new branch already exists", func(t *testing.T) {
		opts := getUpdateRepoFileOptions(repo)
		opts.NewBranch = "develop"
		fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "branch already exists [name: " + opts.NewBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("treePath is empty:", func(t *testing.T) {
		opts := getUpdateRepoFileOptions(repo)
		opts.TreePath = ""
		fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: ]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("treePath is a git directory:", func(t *testing.T) {
		opts := getUpdateRepoFileOptions(repo)
		opts.TreePath = ".git"
		fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: " + opts.TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("create file that already exists", func(t *testing.T) {
		opts := getCreateRepoFileOptions(repo)
		opts.TreePath = "README.md" //already exists
		fileResponse, err := CreateOrUpdateRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "repository file already exists [path: " + opts.TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}
