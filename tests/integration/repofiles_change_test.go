// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/contexttest"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func getCreateRepoFilesOptions(repo *repo_model.Repository) *files_service.ChangeRepoFilesOptions {
	return &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     "create",
				TreePath:      "new/file.txt",
				ContentReader: strings.NewReader("This is a NEW file"),
			},
		},
		OldBranch: repo.DefaultBranch,
		NewBranch: repo.DefaultBranch,
		Message:   "Creates new/file.txt",
		Author:    nil,
		Committer: nil,
	}
}

func getUpdateRepoFilesOptions(repo *repo_model.Repository) *files_service.ChangeRepoFilesOptions {
	return &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     "update",
				TreePath:      "README.md",
				SHA:           "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
				ContentReader: strings.NewReader("This is UPDATED content for the README file"),
			},
		},
		OldBranch: repo.DefaultBranch,
		NewBranch: repo.DefaultBranch,
		Message:   "Updates README.md",
		Author:    nil,
		Committer: nil,
	}
}

func getDeleteRepoFilesOptions(repo *repo_model.Repository) *files_service.ChangeRepoFilesOptions {
	return &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation: "delete",
				TreePath:  "README.md",
				SHA:       "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			},
		},
		LastCommitID: "",
		OldBranch:    repo.DefaultBranch,
		NewBranch:    repo.DefaultBranch,
		Message:      "Deletes README.md",
		Author: &files_service.IdentityOptions{
			Name:  "Bob Smith",
			Email: "bob@smith.com",
		},
		Committer: nil,
	}
}

func getExpectedFileResponseForRepofilesDelete() *api.FileResponse {
	// Just returns fields that don't change, i.e. fields with commit SHAs and dates can't be determined
	return &api.FileResponse{
		Content: nil,
		Commit: &api.FileCommitResponse{
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Bob Smith",
					Email: "bob@smith.com",
				},
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Bob Smith",
					Email: "bob@smith.com",
				},
			},
			Message: "Deletes README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func getExpectedFileResponseForRepofilesCreate(commitID, lastCommitSHA string) *api.FileResponse {
	treePath := "new/file.txt"
	encoding := "base64"
	content := "VGhpcyBpcyBhIE5FVyBmaWxl"
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=master"
	htmlURL := setting.AppURL + "user2/repo1/src/branch/master/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/103ff9234cefeee5ec5361d22b49fbb04d385885"
	downloadURL := setting.AppURL + "user2/repo1/raw/branch/master/" + treePath
	return &api.FileResponse{
		Content: &api.ContentsResponse{
			Name:          filepath.Base(treePath),
			Path:          treePath,
			SHA:           "103ff9234cefeee5ec5361d22b49fbb04d385885",
			LastCommitSHA: lastCommitSHA,
			Type:          "file",
			Size:          18,
			Encoding:      &encoding,
			Content:       &content,
			URL:           &selfURL,
			HTMLURL:       &htmlURL,
			GitURL:        &gitURL,
			DownloadURL:   &downloadURL,
			Links: &api.FileLinksResponse{
				Self:    &selfURL,
				GitURL:  &gitURL,
				HTMLURL: &htmlURL,
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: setting.AppURL + "user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@noreply.example.org",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@noreply.example.org",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Parents: []*api.CommitMeta{
				{
					URL: setting.AppURL + "api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
					SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
				},
			},
			Message: "Updates README.md\n",
			Tree: &api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/user2/repo1/git/trees/f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
				SHA: "f93e3a1a1525fb5b91020git dda86e44810c87a2d7bc",
			},
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func getExpectedFileResponseForRepofilesUpdate(commitID, filename, lastCommitSHA string) *api.FileResponse {
	encoding := "base64"
	content := "VGhpcyBpcyBVUERBVEVEIGNvbnRlbnQgZm9yIHRoZSBSRUFETUUgZmlsZQ=="
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + filename + "?ref=master"
	htmlURL := setting.AppURL + "user2/repo1/src/branch/master/" + filename
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/dbf8d00e022e05b7e5cf7e535de857de57925647"
	downloadURL := setting.AppURL + "user2/repo1/raw/branch/master/" + filename
	return &api.FileResponse{
		Content: &api.ContentsResponse{
			Name:          filename,
			Path:          filename,
			SHA:           "dbf8d00e022e05b7e5cf7e535de857de57925647",
			LastCommitSHA: lastCommitSHA,
			Type:          "file",
			Size:          43,
			Encoding:      &encoding,
			Content:       &content,
			URL:           &selfURL,
			HTMLURL:       &htmlURL,
			GitURL:        &gitURL,
			DownloadURL:   &downloadURL,
			Links: &api.FileLinksResponse{
				Self:    &selfURL,
				GitURL:  &gitURL,
				HTMLURL: &htmlURL,
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: setting.AppURL + "user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@noreply.example.org",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "User Two",
					Email: "user2@noreply.example.org",
				},
				Date: time.Now().UTC().Format(time.RFC3339),
			},
			Parents: []*api.CommitMeta{
				{
					URL: setting.AppURL + "api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
					SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
				},
			},
			Message: "Updates README.md\n",
			Tree: &api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/user2/repo1/git/trees/f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
				SHA: "f93e3a1a1525fb5b91020da86e44810c87a2d7bc",
			},
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func TestChangeRepoFilesForCreate(t *testing.T) {
	// setup
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		ctx, _ := contexttest.MockContext(t, "user2/repo1")
		ctx.SetParams(":id", "1")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()

		repo := ctx.Repo.Repository
		doer := ctx.Doer
		opts := getCreateRepoFilesOptions(repo)

		// test
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)

		// asserts
		assert.NoError(t, err)
		gitRepo, _ := gitrepo.OpenRepository(git.DefaultContext, repo)
		defer gitRepo.Close()

		commitID, _ := gitRepo.GetBranchCommitID(opts.NewBranch)
		lastCommit, _ := gitRepo.GetCommitByPath("new/file.txt")
		expectedFileResponse := getExpectedFileResponseForRepofilesCreate(commitID, lastCommit.ID.String())
		assert.NotNil(t, expectedFileResponse)
		if expectedFileResponse != nil {
			assert.EqualValues(t, expectedFileResponse.Content, filesResponse.Files[0])
			assert.EqualValues(t, expectedFileResponse.Commit.SHA, filesResponse.Commit.SHA)
			assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, filesResponse.Commit.HTMLURL)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, filesResponse.Commit.Author.Email)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, filesResponse.Commit.Author.Name)
		}
	})
}

func TestChangeRepoFilesForUpdate(t *testing.T) {
	// setup
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		ctx, _ := contexttest.MockContext(t, "user2/repo1")
		ctx.SetParams(":id", "1")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()

		repo := ctx.Repo.Repository
		doer := ctx.Doer
		opts := getUpdateRepoFilesOptions(repo)

		// test
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)

		// asserts
		assert.NoError(t, err)
		gitRepo, _ := gitrepo.OpenRepository(git.DefaultContext, repo)
		defer gitRepo.Close()

		commit, _ := gitRepo.GetBranchCommit(opts.NewBranch)
		lastCommit, _ := commit.GetCommitByPath(opts.Files[0].TreePath)
		expectedFileResponse := getExpectedFileResponseForRepofilesUpdate(commit.ID.String(), opts.Files[0].TreePath, lastCommit.ID.String())
		assert.EqualValues(t, expectedFileResponse.Content, filesResponse.Files[0])
		assert.EqualValues(t, expectedFileResponse.Commit.SHA, filesResponse.Commit.SHA)
		assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, filesResponse.Commit.HTMLURL)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, filesResponse.Commit.Author.Email)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, filesResponse.Commit.Author.Name)
	})
}

func TestChangeRepoFilesForUpdateWithFileMove(t *testing.T) {
	// setup
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		ctx, _ := contexttest.MockContext(t, "user2/repo1")
		ctx.SetParams(":id", "1")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()

		repo := ctx.Repo.Repository
		doer := ctx.Doer
		opts := getUpdateRepoFilesOptions(repo)
		opts.Files[0].FromTreePath = "README.md"
		opts.Files[0].TreePath = "README_new.md" // new file name, README_new.md

		// test
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)

		// asserts
		assert.NoError(t, err)
		gitRepo, _ := gitrepo.OpenRepository(git.DefaultContext, repo)
		defer gitRepo.Close()

		commit, _ := gitRepo.GetBranchCommit(opts.NewBranch)
		lastCommit, _ := commit.GetCommitByPath(opts.Files[0].TreePath)
		expectedFileResponse := getExpectedFileResponseForRepofilesUpdate(commit.ID.String(), opts.Files[0].TreePath, lastCommit.ID.String())
		// assert that the old file no longer exists in the last commit of the branch
		fromEntry, err := commit.GetTreeEntryByPath(opts.Files[0].FromTreePath)
		switch err.(type) {
		case git.ErrNotExist:
			// correct, continue
		default:
			t.Fatalf("expected git.ErrNotExist, got:%v", err)
		}
		toEntry, err := commit.GetTreeEntryByPath(opts.Files[0].TreePath)
		assert.NoError(t, err)
		assert.Nil(t, fromEntry)  // Should no longer exist here
		assert.NotNil(t, toEntry) // Should exist here
		// assert SHA has remained the same but paths use the new file name
		assert.EqualValues(t, expectedFileResponse.Content.SHA, filesResponse.Files[0].SHA)
		assert.EqualValues(t, expectedFileResponse.Content.Name, filesResponse.Files[0].Name)
		assert.EqualValues(t, expectedFileResponse.Content.Path, filesResponse.Files[0].Path)
		assert.EqualValues(t, expectedFileResponse.Content.URL, filesResponse.Files[0].URL)
		assert.EqualValues(t, expectedFileResponse.Commit.SHA, filesResponse.Commit.SHA)
		assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, filesResponse.Commit.HTMLURL)
	})
}

// Test opts with branch names removed, should get same results as above test
func TestChangeRepoFilesWithoutBranchNames(t *testing.T) {
	// setup
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		ctx, _ := contexttest.MockContext(t, "user2/repo1")
		ctx.SetParams(":id", "1")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()

		repo := ctx.Repo.Repository
		doer := ctx.Doer
		opts := getUpdateRepoFilesOptions(repo)
		opts.OldBranch = ""
		opts.NewBranch = ""

		// test
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)

		// asserts
		assert.NoError(t, err)
		gitRepo, _ := gitrepo.OpenRepository(git.DefaultContext, repo)
		defer gitRepo.Close()

		commit, _ := gitRepo.GetBranchCommit(repo.DefaultBranch)
		lastCommit, _ := commit.GetCommitByPath(opts.Files[0].TreePath)
		expectedFileResponse := getExpectedFileResponseForRepofilesUpdate(commit.ID.String(), opts.Files[0].TreePath, lastCommit.ID.String())
		assert.EqualValues(t, expectedFileResponse.Content, filesResponse.Files[0])
	})
}

func TestChangeRepoFilesForDelete(t *testing.T) {
	onGiteaRun(t, testDeleteRepoFiles)
}

func testDeleteRepoFiles(t *testing.T, u *url.URL) {
	// setup
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()
	repo := ctx.Repo.Repository
	doer := ctx.Doer
	opts := getDeleteRepoFilesOptions(repo)

	t.Run("Delete README.md file", func(t *testing.T) {
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.NoError(t, err)
		expectedFileResponse := getExpectedFileResponseForRepofilesDelete()
		assert.NotNil(t, filesResponse)
		assert.Nil(t, filesResponse.Files[0])
		assert.EqualValues(t, expectedFileResponse.Commit.Message, filesResponse.Commit.Message)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Identity, filesResponse.Commit.Author.Identity)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Identity, filesResponse.Commit.Committer.Identity)
		assert.EqualValues(t, expectedFileResponse.Verification, filesResponse.Verification)
	})

	t.Run("Verify README.md has been deleted", func(t *testing.T) {
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Nil(t, filesResponse)
		expectedError := "repository file does not exist [path: " + opts.Files[0].TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}

// Test opts with branch names removed, same results
func TestChangeRepoFilesForDeleteWithoutBranchNames(t *testing.T) {
	onGiteaRun(t, testDeleteRepoFilesWithoutBranchNames)
}

func testDeleteRepoFilesWithoutBranchNames(t *testing.T, u *url.URL) {
	// setup
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	doer := ctx.Doer
	opts := getDeleteRepoFilesOptions(repo)
	opts.OldBranch = ""
	opts.NewBranch = ""

	t.Run("Delete README.md without Branch Name", func(t *testing.T) {
		filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.NoError(t, err)
		expectedFileResponse := getExpectedFileResponseForRepofilesDelete()
		assert.NotNil(t, filesResponse)
		assert.Nil(t, filesResponse.Files[0])
		assert.EqualValues(t, expectedFileResponse.Commit.Message, filesResponse.Commit.Message)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Identity, filesResponse.Commit.Author.Identity)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Identity, filesResponse.Commit.Committer.Identity)
		assert.EqualValues(t, expectedFileResponse.Verification, filesResponse.Verification)
	})
}

func TestChangeRepoFilesErrors(t *testing.T) {
	// setup
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		ctx, _ := contexttest.MockContext(t, "user2/repo1")
		ctx.SetParams(":id", "1")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()

		repo := ctx.Repo.Repository
		doer := ctx.Doer

		t.Run("bad branch", func(t *testing.T) {
			opts := getUpdateRepoFilesOptions(repo)
			opts.OldBranch = "bad_branch"
			filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
			assert.Error(t, err)
			assert.Nil(t, filesResponse)
			expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
			assert.EqualError(t, err, expectedError)
		})

		t.Run("bad SHA", func(t *testing.T) {
			opts := getUpdateRepoFilesOptions(repo)
			origSHA := opts.Files[0].SHA
			opts.Files[0].SHA = "bad_sha"
			filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
			assert.Nil(t, filesResponse)
			assert.Error(t, err)
			expectedError := "sha does not match [given: " + opts.Files[0].SHA + ", expected: " + origSHA + "]"
			assert.EqualError(t, err, expectedError)
		})

		t.Run("new branch already exists", func(t *testing.T) {
			opts := getUpdateRepoFilesOptions(repo)
			opts.NewBranch = "develop"
			filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
			assert.Nil(t, filesResponse)
			assert.Error(t, err)
			expectedError := "branch already exists [name: " + opts.NewBranch + "]"
			assert.EqualError(t, err, expectedError)
		})

		t.Run("treePath is empty:", func(t *testing.T) {
			opts := getUpdateRepoFilesOptions(repo)
			opts.Files[0].TreePath = ""
			filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
			assert.Nil(t, filesResponse)
			assert.Error(t, err)
			expectedError := "path contains a malformed path component [path: ]"
			assert.EqualError(t, err, expectedError)
		})

		t.Run("treePath is a git directory:", func(t *testing.T) {
			opts := getUpdateRepoFilesOptions(repo)
			opts.Files[0].TreePath = ".git"
			filesResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
			assert.Nil(t, filesResponse)
			assert.Error(t, err)
			expectedError := "path contains a malformed path component [path: " + opts.Files[0].TreePath + "]"
			assert.EqualError(t, err, expectedError)
		})

		t.Run("create file that already exists", func(t *testing.T) {
			opts := getCreateRepoFilesOptions(repo)
			opts.Files[0].TreePath = "README.md" // already exists
			fileResponse, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, doer, opts)
			assert.Nil(t, fileResponse)
			assert.Error(t, err)
			expectedError := "repository file already exists [path: " + opts.Files[0].TreePath + "]"
			assert.EqualError(t, err, expectedError)
		})
	})
}
