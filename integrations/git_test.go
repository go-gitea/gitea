// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

const (
	littleSize = 1024              //1ko
	bigSize    = 128 * 1024 * 1024 //128Mo
)

func TestGit(t *testing.T) {
	onGiteaRun(t, testGit)
}

func testGit(t *testing.T, u *url.URL) {
	username := "user2"
	baseAPITestContext := NewAPITestContext(t, username, "repo1")

	u.Path = baseAPITestContext.GitPath()

	forkedUserCtx := NewAPITestContext(t, "user4", "repo1")

	t.Run("HTTP", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		ensureAnonymousClone(t, u)
		httpContext := baseAPITestContext
		httpContext.Reponame = "repo-tmp-17"
		forkedUserCtx.Reponame = httpContext.Reponame

		dstPath, err := ioutil.TempDir("", httpContext.Reponame)
		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)

		t.Run("CreateRepoInDifferentUser", doAPICreateRepository(forkedUserCtx, false))
		t.Run("AddUserAsCollaborator", doAPIAddCollaborator(forkedUserCtx, httpContext.Username, models.AccessModeRead))

		t.Run("ForkFromDifferentUser", doAPIForkRepository(httpContext, forkedUserCtx.Username))

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword(username, userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		little, big := standardCommitAndPushTest(t, dstPath)
		littleLFS, bigLFS := lfsCommitAndPushTest(t, dstPath)
		rawTest(t, &httpContext, little, big, littleLFS, bigLFS)
		mediaTest(t, &httpContext, little, big, littleLFS, bigLFS)

		t.Run("BranchProtectMerge", doBranchProtectPRMerge(&httpContext, dstPath))
		t.Run("MergeFork", func(t *testing.T) {
			t.Run("CreatePRAndMerge", doMergeFork(httpContext, forkedUserCtx, "master", httpContext.Username+":master"))
			t.Run("DeleteRepository", doAPIDeleteRepository(httpContext))
			rawTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
			mediaTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
		})

		t.Run("PushCreate", doPushCreate(httpContext, u))
	})
	t.Run("SSH", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		sshContext := baseAPITestContext
		sshContext.Reponame = "repo-tmp-18"
		keyname := "my-testing-key"
		forkedUserCtx.Reponame = sshContext.Reponame
		t.Run("CreateRepoInDifferentUser", doAPICreateRepository(forkedUserCtx, false))
		t.Run("AddUserAsCollaborator", doAPIAddCollaborator(forkedUserCtx, sshContext.Username, models.AccessModeRead))
		t.Run("ForkFromDifferentUser", doAPIForkRepository(sshContext, forkedUserCtx.Username))

		//Setup key the user ssh key
		withKeyFile(t, keyname, func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(sshContext, "test-key", keyFile))

			//Setup remote link
			//TODO: get url from api
			sshURL := createSSHUrl(sshContext.GitPath(), u)

			//Setup clone folder
			dstPath, err := ioutil.TempDir("", sshContext.Reponame)
			assert.NoError(t, err)
			defer os.RemoveAll(dstPath)

			t.Run("Clone", doGitClone(dstPath, sshURL))

			little, big := standardCommitAndPushTest(t, dstPath)
			littleLFS, bigLFS := lfsCommitAndPushTest(t, dstPath)
			rawTest(t, &sshContext, little, big, littleLFS, bigLFS)
			mediaTest(t, &sshContext, little, big, littleLFS, bigLFS)

			t.Run("BranchProtectMerge", doBranchProtectPRMerge(&sshContext, dstPath))
			t.Run("MergeFork", func(t *testing.T) {
				t.Run("CreatePRAndMerge", doMergeFork(sshContext, forkedUserCtx, "master", sshContext.Username+":master"))
				t.Run("DeleteRepository", doAPIDeleteRepository(sshContext))
				rawTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
				mediaTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
			})

			t.Run("PushCreate", doPushCreate(sshContext, sshURL))
		})
	})
}

func ensureAnonymousClone(t *testing.T, u *url.URL) {
	dstLocalPath, err := ioutil.TempDir("", "repo1")
	assert.NoError(t, err)
	defer os.RemoveAll(dstLocalPath)
	t.Run("CloneAnonymous", doGitClone(dstLocalPath, u))

}

func standardCommitAndPushTest(t *testing.T, dstPath string) (little, big string) {
	t.Run("Standard", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		little, big = commitAndPushTest(t, dstPath, "data-file-")
	})
	return
}

func lfsCommitAndPushTest(t *testing.T, dstPath string) (littleLFS, bigLFS string) {
	t.Run("LFS", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		setting.CheckLFSVersion()
		if !setting.LFS.StartServer {
			t.Skip()
			return
		}
		prefix := "lfs-data-file-"
		_, err := git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
		assert.NoError(t, err)
		_, err = git.NewCommand("lfs").AddArguments("track", prefix+"*").RunInDir(dstPath)
		assert.NoError(t, err)
		err = git.AddChanges(dstPath, false, ".gitattributes")
		assert.NoError(t, err)

		err = git.CommitChangesWithArgs(dstPath, allowLFSFilters(), git.CommitChangesOptions{
			Committer: &git.Signature{
				Email: "user2@example.com",
				Name:  "User Two",
				When:  time.Now(),
			},
			Author: &git.Signature{
				Email: "user2@example.com",
				Name:  "User Two",
				When:  time.Now(),
			},
			Message: fmt.Sprintf("Testing commit @ %v", time.Now()),
		})
		assert.NoError(t, err)

		littleLFS, bigLFS = commitAndPushTest(t, dstPath, prefix)

		t.Run("Locks", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			lockTest(t, dstPath)
		})
	})
	return
}

func commitAndPushTest(t *testing.T, dstPath, prefix string) (little, big string) {
	t.Run("PushCommit", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		t.Run("Little", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			little = doCommitAndPush(t, littleSize, dstPath, prefix)
		})
		t.Run("Big", func(t *testing.T) {
			if testing.Short() {
				t.Skip("Skipping test in short mode.")
				return
			}
			defer PrintCurrentTest(t)()
			big = doCommitAndPush(t, bigSize, dstPath, prefix)
		})
	})
	return
}

func rawTest(t *testing.T, ctx *APITestContext, little, big, littleLFS, bigLFS string) {
	t.Run("Raw", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		username := ctx.Username
		reponame := ctx.Reponame

		session := loginUser(t, username)

		// Request raw paths
		req := NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", little))
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, littleSize, resp.Body.Len())

		setting.CheckLFSVersion()
		if setting.LFS.StartServer {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", littleLFS))
			resp = session.MakeRequest(t, req, http.StatusOK)
			assert.NotEqual(t, littleSize, resp.Body.Len())
			assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)
		}

		if !testing.Short() {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", big))
			resp = session.MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, bigSize, resp.Body.Len())

			if setting.LFS.StartServer {
				req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", bigLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, bigSize, resp.Body.Len())
				assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)
			}
		}
	})
}

func mediaTest(t *testing.T, ctx *APITestContext, little, big, littleLFS, bigLFS string) {
	t.Run("Media", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		username := ctx.Username
		reponame := ctx.Reponame

		session := loginUser(t, username)

		// Request media paths
		req := NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", little))
		resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
		assert.Equal(t, littleSize, resp.Length)

		setting.CheckLFSVersion()
		if setting.LFS.StartServer {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", littleLFS))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Length)
		}

		if !testing.Short() {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", big))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, bigSize, resp.Length)

			if setting.LFS.StartServer {
				req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", bigLFS))
				resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Length)
			}
		}
	})
}

func lockTest(t *testing.T, repoPath string) {
	lockFileTest(t, "README.md", repoPath)
}

func lockFileTest(t *testing.T, filename, repoPath string) {
	_, err := git.NewCommand("lfs").AddArguments("locks").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("lock", filename).RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("locks").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("unlock", filename).RunInDir(repoPath)
	assert.NoError(t, err)
}

func doCommitAndPush(t *testing.T, size int, repoPath, prefix string) string {
	name, err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two", prefix)
	assert.NoError(t, err)
	_, err = git.NewCommand("push", "origin", "master").RunInDir(repoPath) //Push
	assert.NoError(t, err)
	return name
}

func generateCommitWithNewData(size int, repoPath, email, fullName, prefix string) (string, error) {
	//Generate random file
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	tmpFile, err := ioutil.TempFile(repoPath, prefix)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	_, err = tmpFile.Write(data)
	if err != nil {
		return "", err
	}

	//Commit
	// Now here we should explicitly allow lfs filters to run
	globalArgs := allowLFSFilters()
	err = git.AddChangesWithArgs(repoPath, globalArgs, false, filepath.Base(tmpFile.Name()))
	if err != nil {
		return "", err
	}
	err = git.CommitChangesWithArgs(repoPath, globalArgs, git.CommitChangesOptions{
		Committer: &git.Signature{
			Email: email,
			Name:  fullName,
			When:  time.Now(),
		},
		Author: &git.Signature{
			Email: email,
			Name:  fullName,
			When:  time.Now(),
		},
		Message: fmt.Sprintf("Testing commit @ %v", time.Now()),
	})
	return filepath.Base(tmpFile.Name()), err
}

func doBranchProtectPRMerge(baseCtx *APITestContext, dstPath string) func(t *testing.T) {
	return func(t *testing.T) {
		defer PrintCurrentTest(t)()
		t.Run("CreateBranchProtected", doGitCreateBranch(dstPath, "protected"))
		t.Run("PushProtectedBranch", doGitPushTestRepository(dstPath, "origin", "protected"))

		ctx := NewAPITestContext(t, baseCtx.Username, baseCtx.Reponame)
		t.Run("ProtectProtectedBranchNoWhitelist", doProtectBranch(ctx, "protected", ""))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(littleSize, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("FailToPushToProtectedBranch", doGitPushTestRepositoryFail(dstPath, "origin", "protected"))
		t.Run("PushToUnprotectedBranch", doGitPushTestRepository(dstPath, "origin", "protected:unprotected"))
		var pr api.PullRequest
		var err error
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, "protected", "unprotected")(t)
			assert.NoError(t, err)
		})
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(littleSize, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("PushToUnprotectedBranch", doGitPushTestRepository(dstPath, "origin", "protected:unprotected-2"))
		var pr2 api.PullRequest
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr2, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, "unprotected", "unprotected-2")(t)
			assert.NoError(t, err)
		})
		t.Run("MergePR2", doAPIMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr2.Index))
		t.Run("MergePR", doAPIMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))
		t.Run("PullProtected", doGitPull(dstPath, "origin", "protected"))
		t.Run("ProtectProtectedBranchWhitelist", doProtectBranch(ctx, "protected", baseCtx.Username))

		t.Run("CheckoutMaster", doGitCheckoutBranch(dstPath, "master"))
		t.Run("CreateBranchForced", doGitCreateBranch(dstPath, "toforce"))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(littleSize, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("FailToForcePushToProtectedBranch", doGitPushTestRepositoryFail(dstPath, "-f", "origin", "toforce:protected"))
		t.Run("MergeProtectedToToforce", doGitMerge(dstPath, "protected"))
		t.Run("PushToProtectedBranch", doGitPushTestRepository(dstPath, "origin", "toforce:protected"))
		t.Run("CheckoutMasterAgain", doGitCheckoutBranch(dstPath, "master"))
	}
}

func doProtectBranch(ctx APITestContext, branch string, userToWhitelist string) func(t *testing.T) {
	// We are going to just use the owner to set the protection.
	return func(t *testing.T) {
		csrf := GetCSRF(t, ctx.Session, fmt.Sprintf("/%s/%s/settings/branches", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)))

		if userToWhitelist == "" {
			// Change branch to protected
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/branches/%s", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), url.PathEscape(branch)), map[string]string{
				"_csrf":     csrf,
				"protected": "on",
			})
			ctx.Session.MakeRequest(t, req, http.StatusFound)
		} else {
			user, err := models.GetUserByName(userToWhitelist)
			assert.NoError(t, err)
			// Change branch to protected
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/branches/%s", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), url.PathEscape(branch)), map[string]string{
				"_csrf":            csrf,
				"protected":        "on",
				"enable_push":      "whitelist",
				"enable_whitelist": "on",
				"whitelist_users":  strconv.FormatInt(user.ID, 10),
			})
			ctx.Session.MakeRequest(t, req, http.StatusFound)
		}
		// Check if master branch has been locked successfully
		flashCookie := ctx.Session.GetCookie("macaron_flash")
		assert.NotNil(t, flashCookie)
		assert.EqualValues(t, "success%3DBranch%2Bprotection%2Bfor%2Bbranch%2B%2527"+url.QueryEscape(branch)+"%2527%2Bhas%2Bbeen%2Bupdated.", flashCookie.Value)
	}
}

func doMergeFork(ctx, baseCtx APITestContext, baseBranch, headBranch string) func(t *testing.T) {
	return func(t *testing.T) {
		var pr api.PullRequest
		var err error
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, baseBranch, headBranch)(t)
			assert.NoError(t, err)
		})
		t.Run("MergePR", doAPIMergePullRequest(baseCtx, baseCtx.Username, baseCtx.Reponame, pr.Index))

	}
}

func doPushCreate(ctx APITestContext, u *url.URL) func(t *testing.T) {
	return func(t *testing.T) {
		defer PrintCurrentTest(t)()
		ctx.Reponame = fmt.Sprintf("repo-tmp-push-create-%s", u.Scheme)
		u.Path = ctx.GitPath()

		tmpDir, err := ioutil.TempDir("", ctx.Reponame)
		assert.NoError(t, err)

		_, err = git.NewCommand("clone", u.String()).RunInDir(tmpDir)
		assert.Error(t, err)

		err = git.InitRepository(tmpDir, false)
		assert.NoError(t, err)

		_, err = os.Create(filepath.Join(tmpDir, "test.txt"))
		assert.NoError(t, err)

		err = git.AddChanges(tmpDir, true)
		assert.NoError(t, err)

		err = git.CommitChanges(tmpDir, git.CommitChangesOptions{
			Committer: &git.Signature{
				Email: "user2@example.com",
				Name:  "User Two",
				When:  time.Now(),
			},
			Author: &git.Signature{
				Email: "user2@example.com",
				Name:  "User Two",
				When:  time.Now(),
			},
			Message: fmt.Sprintf("Testing push create @ %v", time.Now()),
		})
		assert.NoError(t, err)

		_, err = git.NewCommand("remote", "add", "origin", u.String()).RunInDir(tmpDir)
		assert.NoError(t, err)

		invalidCtx := ctx
		invalidCtx.Reponame = fmt.Sprintf("invalid/repo-tmp-push-create-%s", u.Scheme)
		u.Path = invalidCtx.GitPath()

		_, err = git.NewCommand("remote", "add", "invalid", u.String()).RunInDir(tmpDir)
		assert.NoError(t, err)

		// Push to create disabled
		setting.Repository.EnablePushCreateUser = false
		_, err = git.NewCommand("push", "origin", "master").RunInDir(tmpDir)
		assert.Error(t, err)

		// Push to create enabled
		setting.Repository.EnablePushCreateUser = true

		// Invalid repo
		_, err = git.NewCommand("push", "invalid", "master").RunInDir(tmpDir)
		assert.Error(t, err)

		// Valid repo
		_, err = git.NewCommand("push", "origin", "master").RunInDir(tmpDir)
		assert.NoError(t, err)

		// Fetch repo from database
		repo, err := models.GetRepositoryByOwnerAndName(ctx.Username, ctx.Reponame)
		assert.NoError(t, err)
		assert.False(t, repo.IsEmpty)
		assert.True(t, repo.IsPrivate)
	}
}
