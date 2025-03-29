// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/hex"
	"fmt"
	"io"
	mathRand "math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFileSizeSmall = 10
	testFileSizeLarge = 10 * 1024 * 1024 // 10M
)

func TestGitGeneral(t *testing.T) {
	onGiteaRun(t, testGitGeneral)
}

func testGitGeneral(t *testing.T, u *url.URL) {
	username := "user2"
	baseAPITestContext := NewAPITestContext(t, username, "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	u.Path = baseAPITestContext.GitPath()

	forkedUserCtx := NewAPITestContext(t, "user4", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	t.Run("HTTP", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		ensureAnonymousClone(t, u)
		httpContext := baseAPITestContext
		httpContext.Reponame = "repo-tmp-17"
		forkedUserCtx.Reponame = httpContext.Reponame

		dstPath := t.TempDir()

		t.Run("CreateRepoInDifferentUser", doAPICreateRepository(forkedUserCtx, false))
		t.Run("AddUserAsCollaborator", doAPIAddCollaborator(forkedUserCtx, httpContext.Username, perm.AccessModeRead))

		t.Run("ForkFromDifferentUser", doAPIForkRepository(httpContext, forkedUserCtx.Username))

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword(username, userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		dstPath2 := t.TempDir()

		t.Run("Partial Clone", doPartialGitClone(dstPath2, u))

		pushedFilesStandard := standardCommitAndPushTest(t, dstPath, testFileSizeSmall, testFileSizeLarge)
		pushedFilesLFS := lfsCommitAndPushTest(t, dstPath, testFileSizeSmall, testFileSizeLarge)
		rawTest(t, &httpContext, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])
		mediaTest(t, &httpContext, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])

		t.Run("CreateAgitFlowPull", doCreateAgitFlowPull(dstPath, &httpContext, "test/head"))
		t.Run("CreateProtectedBranch", doCreateProtectedBranch(&httpContext, dstPath))
		t.Run("BranchProtectMerge", doBranchProtectPRMerge(&httpContext, dstPath))
		t.Run("AutoMerge", doAutoPRMerge(&httpContext, dstPath))
		t.Run("CreatePRAndSetManuallyMerged", doCreatePRAndSetManuallyMerged(httpContext, httpContext, dstPath, "master", "test-manually-merge"))
		t.Run("MergeFork", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			t.Run("CreatePRAndMerge", doMergeFork(httpContext, forkedUserCtx, "master", httpContext.Username+":master"))
			rawTest(t, &forkedUserCtx, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])
			mediaTest(t, &forkedUserCtx, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])
		})

		t.Run("PushCreate", doPushCreate(httpContext, u))
	})
	t.Run("SSH", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		sshContext := baseAPITestContext
		sshContext.Reponame = "repo-tmp-18"
		keyname := "my-testing-key"
		forkedUserCtx.Reponame = sshContext.Reponame
		t.Run("CreateRepoInDifferentUser", doAPICreateRepository(forkedUserCtx, false))
		t.Run("AddUserAsCollaborator", doAPIAddCollaborator(forkedUserCtx, sshContext.Username, perm.AccessModeRead))
		t.Run("ForkFromDifferentUser", doAPIForkRepository(sshContext, forkedUserCtx.Username))

		// Setup key the user ssh key
		withKeyFile(t, keyname, func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(sshContext, "test-key", keyFile))

			// Setup remote link
			// TODO: get url from api
			sshURL := createSSHUrl(sshContext.GitPath(), u)

			// Setup clone folder
			dstPath := t.TempDir()

			t.Run("Clone", doGitClone(dstPath, sshURL))

			pushedFilesStandard := standardCommitAndPushTest(t, dstPath, testFileSizeSmall, testFileSizeLarge)
			pushedFilesLFS := lfsCommitAndPushTest(t, dstPath, testFileSizeSmall, testFileSizeLarge)
			rawTest(t, &sshContext, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])
			mediaTest(t, &sshContext, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])

			t.Run("CreateAgitFlowPull", doCreateAgitFlowPull(dstPath, &sshContext, "test/head2"))
			t.Run("CreateProtectedBranch", doCreateProtectedBranch(&sshContext, dstPath))
			t.Run("BranchProtectMerge", doBranchProtectPRMerge(&sshContext, dstPath))
			t.Run("MergeFork", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				t.Run("CreatePRAndMerge", doMergeFork(sshContext, forkedUserCtx, "master", sshContext.Username+":master"))
				rawTest(t, &forkedUserCtx, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])
				mediaTest(t, &forkedUserCtx, pushedFilesStandard[0], pushedFilesStandard[1], pushedFilesLFS[0], pushedFilesLFS[1])
			})

			t.Run("PushCreate", doPushCreate(sshContext, sshURL))
		})
	})
}

func ensureAnonymousClone(t *testing.T, u *url.URL) {
	dstLocalPath := t.TempDir()
	t.Run("CloneAnonymous", doGitClone(dstLocalPath, u))
}

func standardCommitAndPushTest(t *testing.T, dstPath string, sizes ...int) (pushedFiles []string) {
	t.Run("CommitAndPushStandard", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		pushedFiles = commitAndPushTest(t, dstPath, "data-file-", sizes...)
	})
	return pushedFiles
}

func lfsCommitAndPushTest(t *testing.T, dstPath string, sizes ...int) (pushedFiles []string) {
	t.Run("CommitAndPushLFS", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		prefix := "lfs-data-file-"
		err := git.NewCommand("lfs").AddArguments("install").Run(git.DefaultContext, &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		_, _, err = git.NewCommand("lfs").AddArguments("track").AddDynamicArguments(prefix+"*").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		err = git.AddChanges(dstPath, false, ".gitattributes")
		assert.NoError(t, err)

		err = git.CommitChangesWithArgs(dstPath, git.AllowLFSFiltersArgs(), git.CommitChangesOptions{
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

		pushedFiles = commitAndPushTest(t, dstPath, prefix, sizes...)
		t.Run("Locks", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			lockTest(t, dstPath)
		})
	})
	return pushedFiles
}

func commitAndPushTest(t *testing.T, dstPath, prefix string, sizes ...int) (pushedFiles []string) {
	for _, size := range sizes {
		t.Run("PushCommit Size-"+strconv.Itoa(size), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			pushedFiles = append(pushedFiles, doCommitAndPush(t, size, dstPath, prefix))
		})
	}
	return pushedFiles
}

func rawTest(t *testing.T, ctx *APITestContext, little, big, littleLFS, bigLFS string) {
	t.Run("Raw", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		username := ctx.Username
		reponame := ctx.Reponame

		session := loginUser(t, username)

		// Request raw paths
		req := NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", little))
		resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
		assert.Equal(t, testFileSizeSmall, resp.Length)

		if setting.LFS.StartServer {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", littleLFS))
			resp := session.MakeRequest(t, req, http.StatusOK)
			assert.NotEqual(t, testFileSizeSmall, resp.Body.Len())
			assert.LessOrEqual(t, resp.Body.Len(), 1024)
			if resp.Body.Len() != testFileSizeSmall && resp.Body.Len() <= 1024 {
				assert.Contains(t, resp.Body.String(), lfs.MetaFileIdentifier)
			}
		}

		if !testing.Short() {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", big))
			resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, testFileSizeLarge, resp.Length)

			if setting.LFS.StartServer {
				req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", bigLFS))
				resp := session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, testFileSizeLarge, resp.Body.Len())
				if resp.Body.Len() != testFileSizeLarge && resp.Body.Len() <= 1024 {
					assert.Contains(t, resp.Body.String(), lfs.MetaFileIdentifier)
				}
			}
		}
	})
}

func mediaTest(t *testing.T, ctx *APITestContext, little, big, littleLFS, bigLFS string) {
	t.Run("Media", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		username := ctx.Username
		reponame := ctx.Reponame

		session := loginUser(t, username)

		// Request media paths
		req := NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", little))
		resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
		assert.Equal(t, testFileSizeSmall, resp.Length)

		req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", littleLFS))
		resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
		assert.Equal(t, testFileSizeSmall, resp.Length)

		if !testing.Short() {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", big))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, testFileSizeLarge, resp.Length)

			if setting.LFS.StartServer {
				req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", bigLFS))
				resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
				assert.Equal(t, testFileSizeLarge, resp.Length)
			}
		}
	})
}

func lockTest(t *testing.T, repoPath string) {
	lockFileTest(t, "README.md", repoPath)
}

func lockFileTest(t *testing.T, filename, repoPath string) {
	_, _, err := git.NewCommand("lfs").AddArguments("locks").RunStdString(git.DefaultContext, &git.RunOpts{Dir: repoPath})
	assert.NoError(t, err)
	_, _, err = git.NewCommand("lfs").AddArguments("lock").AddDynamicArguments(filename).RunStdString(git.DefaultContext, &git.RunOpts{Dir: repoPath})
	assert.NoError(t, err)
	_, _, err = git.NewCommand("lfs").AddArguments("locks").RunStdString(git.DefaultContext, &git.RunOpts{Dir: repoPath})
	assert.NoError(t, err)
	_, _, err = git.NewCommand("lfs").AddArguments("unlock").AddDynamicArguments(filename).RunStdString(git.DefaultContext, &git.RunOpts{Dir: repoPath})
	assert.NoError(t, err)
}

func doCommitAndPush(t *testing.T, size int, repoPath, prefix string) string {
	name, err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two", prefix)
	assert.NoError(t, err)
	_, _, err = git.NewCommand("push", "origin", "master").RunStdString(git.DefaultContext, &git.RunOpts{Dir: repoPath}) // Push
	assert.NoError(t, err)
	return name
}

func generateCommitWithNewData(size int, repoPath, email, fullName, prefix string) (string, error) {
	tmpFile, err := os.CreateTemp(repoPath, prefix)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	var seed [32]byte
	rander := mathRand.NewChaCha8(seed) // for testing only, no need to seed
	_, err = io.CopyN(tmpFile, rander, int64(size))
	if err != nil {
		return "", err
	}
	_ = tmpFile.Close()

	// Commit
	// Now here we should explicitly allow lfs filters to run
	globalArgs := git.AllowLFSFiltersArgs()
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

func doCreateProtectedBranch(baseCtx *APITestContext, dstPath string) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		ctx := NewAPITestContext(t, baseCtx.Username, baseCtx.Reponame, auth_model.AccessTokenScopeWriteRepository)

		t.Run("ProtectBranchWithFilePatterns", doProtectBranch(ctx, "release-*", baseCtx.Username, "", "", "config*"))

		// push a new branch without any new commits
		t.Run("CreateProtectedBranch-NoChanges", doGitCreateBranch(dstPath, "release-v1.0"))
		t.Run("PushProtectedBranch-NoChanges", doGitPushTestRepository(dstPath, "origin", "release-v1.0"))
		t.Run("CheckoutMaster-NoChanges", doGitCheckoutBranch(dstPath, "master"))

		// push a new branch with a new unprotected file
		t.Run("CreateProtectedBranch-UnprotectedFile", doGitCreateBranch(dstPath, "release-v2.0"))
		_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "abc.txt")
		assert.NoError(t, err)
		t.Run("PushProtectedBranch-UnprotectedFile", doGitPushTestRepository(dstPath, "origin", "release-v2.0"))
		t.Run("CheckoutMaster-UnprotectedFile", doGitCheckoutBranch(dstPath, "master"))

		// push a new branch with a new protected file
		t.Run("CreateProtectedBranch-ProtectedFile", doGitCreateBranch(dstPath, "release-v3.0"))
		_, err = generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "config")
		assert.NoError(t, err)
		t.Run("PushProtectedBranch-ProtectedFile", doGitPushTestRepositoryFail(dstPath, "origin", "release-v3.0"))
		t.Run("CheckoutMaster-ProtectedFile", doGitCheckoutBranch(dstPath, "master"))
	}
}

func doBranchProtectPRMerge(baseCtx *APITestContext, dstPath string) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		t.Run("CreateBranchProtected", doGitCreateBranch(dstPath, "protected"))
		t.Run("PushProtectedBranch", doGitPushTestRepository(dstPath, "origin", "protected"))

		ctx := NewAPITestContext(t, baseCtx.Username, baseCtx.Reponame, auth_model.AccessTokenScopeWriteRepository)

		// Protect branch without any whitelisting
		t.Run("ProtectBranchNoWhitelist", doProtectBranch(ctx, "protected", "", "", "", ""))

		// Try to push without permissions, which should fail
		t.Run("TryPushWithoutPermissions", func(t *testing.T) {
			_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
			doGitPushTestRepositoryFail(dstPath, "origin", "protected")(t)
		})

		// Set up permissions for normal push but not force push
		t.Run("SetupNormalPushPermissions", doProtectBranch(ctx, "protected", baseCtx.Username, "", "", ""))

		// Normal push should work
		t.Run("NormalPushWithPermissions", func(t *testing.T) {
			_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
			doGitPushTestRepository(dstPath, "origin", "protected")(t)
		})

		// Try to force push without force push permissions, which should fail
		t.Run("ForcePushWithoutForcePermissions", func(t *testing.T) {
			t.Run("CreateDivergentHistory", func(t *testing.T) {
				git.NewCommand("reset", "--hard", "HEAD~1").Run(git.DefaultContext, &git.RunOpts{Dir: dstPath})
				_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-new")
				assert.NoError(t, err)
			})
			doGitPushTestRepositoryFail(dstPath, "-f", "origin", "protected")(t)
		})

		// Set up permissions for force push but not normal push
		t.Run("SetupForcePushPermissions", doProtectBranch(ctx, "protected", "", baseCtx.Username, "", ""))

		// Try to force push without normal push permissions, which should fail
		t.Run("ForcePushWithoutNormalPermissions", doGitPushTestRepositoryFail(dstPath, "-f", "origin", "protected"))

		// Set up permissions for normal and force push (both are required to force push)
		t.Run("SetupNormalAndForcePushPermissions", doProtectBranch(ctx, "protected", baseCtx.Username, baseCtx.Username, "", ""))

		// Force push should now work
		t.Run("ForcePushWithPermissions", doGitPushTestRepository(dstPath, "-f", "origin", "protected"))

		t.Run("ProtectProtectedBranchNoWhitelist", doProtectBranch(ctx, "protected", "", "", "", ""))
		t.Run("PushToUnprotectedBranch", doGitPushTestRepository(dstPath, "origin", "protected:unprotected"))
		var pr api.PullRequest
		var err error
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, "protected", "unprotected")(t)
			assert.NoError(t, err)
		})
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
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

		t.Run("ProtectProtectedBranchUnprotectedFilePaths", doProtectBranch(ctx, "protected", "", "", "unprotected-file-*", ""))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "unprotected-file-")
			assert.NoError(t, err)
		})
		t.Run("PushUnprotectedFilesToProtectedBranch", doGitPushTestRepository(dstPath, "origin", "protected"))

		t.Run("ProtectProtectedBranchWhitelist", doProtectBranch(ctx, "protected", baseCtx.Username, "", "", ""))

		t.Run("CheckoutMaster", doGitCheckoutBranch(dstPath, "master"))
		t.Run("CreateBranchForced", doGitCreateBranch(dstPath, "toforce"))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("FailToForcePushToProtectedBranch", doGitPushTestRepositoryFail(dstPath, "-f", "origin", "toforce:protected"))
		t.Run("MergeProtectedToToforce", doGitMerge(dstPath, "protected"))
		t.Run("PushToProtectedBranch", doGitPushTestRepository(dstPath, "origin", "toforce:protected"))
		t.Run("CheckoutMasterAgain", doGitCheckoutBranch(dstPath, "master"))
	}
}

func doProtectBranch(ctx APITestContext, branch, userToWhitelistPush, userToWhitelistForcePush, unprotectedFilePatterns, protectedFilePatterns string) func(t *testing.T) {
	// We are going to just use the owner to set the protection.
	return func(t *testing.T) {
		csrf := GetUserCSRFToken(t, ctx.Session)

		formData := map[string]string{
			"_csrf":                     csrf,
			"rule_name":                 branch,
			"unprotected_file_patterns": unprotectedFilePatterns,
			"protected_file_patterns":   protectedFilePatterns,
		}

		if userToWhitelistPush != "" {
			user, err := user_model.GetUserByName(db.DefaultContext, userToWhitelistPush)
			assert.NoError(t, err)
			formData["whitelist_users"] = strconv.FormatInt(user.ID, 10)
			formData["enable_push"] = "whitelist"
			formData["enable_whitelist"] = "on"
		}

		if userToWhitelistForcePush != "" {
			user, err := user_model.GetUserByName(db.DefaultContext, userToWhitelistForcePush)
			assert.NoError(t, err)
			formData["force_push_allowlist_users"] = strconv.FormatInt(user.ID, 10)
			formData["enable_force_push"] = "whitelist"
			formData["enable_force_push_allowlist"] = "on"
		}

		// Send the request to update branch protection settings
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/branches/edit", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)), formData)
		ctx.Session.MakeRequest(t, req, http.StatusSeeOther)

		// Check if master branch has been locked successfully
		flashMsg := ctx.Session.GetCookieFlashMessage()
		assert.EqualValues(t, `Branch protection for rule "`+branch+`" has been updated.`, flashMsg.SuccessMsg)
	}
}

func doMergeFork(ctx, baseCtx APITestContext, baseBranch, headBranch string) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		var pr api.PullRequest
		var err error

		// Create a test pullrequest
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, baseBranch, headBranch)(t)
			assert.NoError(t, err)
		})

		// Ensure the PR page works
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))

		// Then get the diff string
		var diffHash string
		var diffLength int
		t.Run("GetDiff", func(t *testing.T) {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d.diff", url.PathEscape(baseCtx.Username), url.PathEscape(baseCtx.Reponame), pr.Index))
			resp := ctx.Session.MakeRequestNilResponseHashSumRecorder(t, req, http.StatusOK)
			diffHash = string(resp.Hash.Sum(nil))
			diffLength = resp.Length
		})

		// Now: Merge the PR & make sure that doesn't break the PR page or change its diff
		t.Run("MergePR", doAPIMergePullRequest(baseCtx, baseCtx.Username, baseCtx.Reponame, pr.Index))
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))
		t.Run("CheckPR", func(t *testing.T) {
			oldMergeBase := pr.MergeBase
			pr2, err := doAPIGetPullRequest(baseCtx, baseCtx.Username, baseCtx.Reponame, pr.Index)(t)
			assert.NoError(t, err)
			assert.Equal(t, oldMergeBase, pr2.MergeBase)
		})
		t.Run("EnsurDiffNoChange", doEnsureDiffNoChange(baseCtx, pr, diffHash, diffLength))

		// Then: Delete the head branch & make sure that doesn't break the PR page or change its diff
		t.Run("DeleteHeadBranch", doBranchDelete(baseCtx, baseCtx.Username, baseCtx.Reponame, headBranch))
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))
		t.Run("EnsureDiffNoChange", doEnsureDiffNoChange(baseCtx, pr, diffHash, diffLength))

		// Delete the head repository & make sure that doesn't break the PR page or change its diff
		t.Run("DeleteHeadRepository", doAPIDeleteRepository(ctx))
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))
		t.Run("EnsureDiffNoChange", doEnsureDiffNoChange(baseCtx, pr, diffHash, diffLength))
	}
}

func doCreatePRAndSetManuallyMerged(ctx, baseCtx APITestContext, dstPath, baseBranch, headBranch string) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		var (
			pr           api.PullRequest
			err          error
			lastCommitID string
		)

		trueBool := true
		falseBool := false

		t.Run("AllowSetManuallyMergedAndSwitchOffAutodetectManualMerge", doAPIEditRepository(baseCtx, &api.EditRepoOption{
			HasPullRequests:       &trueBool,
			AllowManualMerge:      &trueBool,
			AutodetectManualMerge: &falseBool,
		}))

		t.Run("CreateHeadBranch", doGitCreateBranch(dstPath, headBranch))
		t.Run("PushToHeadBranch", doGitPushTestRepository(dstPath, "origin", headBranch))
		t.Run("CreateEmptyPullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, baseBranch, headBranch)(t)
			assert.NoError(t, err)
		})
		lastCommitID = pr.Base.Sha
		t.Run("ManuallyMergePR", doAPIManuallyMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, lastCommitID, pr.Index))
	}
}

func doEnsureCanSeePull(ctx APITestContext, pr api.PullRequest) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		ctx.Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d/files", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		ctx.Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d/commits", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		ctx.Session.MakeRequest(t, req, http.StatusOK)
	}
}

func doEnsureDiffNoChange(ctx APITestContext, pr api.PullRequest, diffHash string, diffLength int) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d.diff", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		resp := ctx.Session.MakeRequestNilResponseHashSumRecorder(t, req, http.StatusOK)
		actual := string(resp.Hash.Sum(nil))
		actualLength := resp.Length

		equal := diffHash == actual
		assert.True(t, equal, "Unexpected change in the diff string: expected hash: %s size: %d but was actually: %s size: %d", hex.EncodeToString([]byte(diffHash)), diffLength, hex.EncodeToString([]byte(actual)), actualLength)
	}
}

func doPushCreate(ctx APITestContext, u *url.URL) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// create a context for a currently non-existent repository
		ctx.Reponame = fmt.Sprintf("repo-tmp-push-create-%s", u.Scheme)
		u.Path = ctx.GitPath()

		// Create a temporary directory
		tmpDir := t.TempDir()

		// Now create local repository to push as our test and set its origin
		t.Run("InitTestRepository", doGitInitTestRepository(tmpDir))
		t.Run("AddRemote", doGitAddRemote(tmpDir, "origin", u))

		// Disable "Push To Create" and attempt to push
		setting.Repository.EnablePushCreateUser = false
		t.Run("FailToPushAndCreateTestRepository", doGitPushTestRepositoryFail(tmpDir, "origin", "master"))

		// Enable "Push To Create"
		setting.Repository.EnablePushCreateUser = true

		// Assert that cloning from a non-existent repository does not create it and that it definitely wasn't create above
		t.Run("FailToCloneFromNonExistentRepository", doGitCloneFail(u))

		// Then "Push To Create"x
		t.Run("SuccessfullyPushAndCreateTestRepository", doGitPushTestRepository(tmpDir, "origin", "master"))

		// Finally, fetch repo from database and ensure the correct repository has been created
		repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, ctx.Username, ctx.Reponame)
		assert.NoError(t, err)
		assert.False(t, repo.IsEmpty)
		assert.True(t, repo.IsPrivate)

		// Now add a remote that is invalid to "Push To Create"
		invalidCtx := ctx
		invalidCtx.Reponame = fmt.Sprintf("invalid/repo-tmp-push-create-%s", u.Scheme)
		u.Path = invalidCtx.GitPath()
		t.Run("AddInvalidRemote", doGitAddRemote(tmpDir, "invalid", u))

		// Fail to "Push To Create" the invalid
		t.Run("FailToPushAndCreateInvalidTestRepository", doGitPushTestRepositoryFail(tmpDir, "invalid", "master"))
	}
}

func doBranchDelete(ctx APITestContext, owner, repo, branch string) func(*testing.T) {
	return func(t *testing.T) {
		csrf := GetUserCSRFToken(t, ctx.Session)

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/branches/delete?name=%s", url.PathEscape(owner), url.PathEscape(repo), url.QueryEscape(branch)), map[string]string{
			"_csrf": csrf,
		})
		ctx.Session.MakeRequest(t, req, http.StatusOK)
	}
}

func doAutoPRMerge(baseCtx *APITestContext, dstPath string) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		ctx := NewAPITestContext(t, baseCtx.Username, baseCtx.Reponame, auth_model.AccessTokenScopeWriteRepository)

		t.Run("CheckoutProtected", doGitCheckoutBranch(dstPath, "protected"))
		t.Run("PullProtected", doGitPull(dstPath, "origin", "protected"))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("PushToUnprotectedBranch", doGitPushTestRepository(dstPath, "origin", "protected:unprotected3"))
		var pr api.PullRequest
		var err error
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, "protected", "unprotected3")(t)
			assert.NoError(t, err)
		})

		// Request repository commits page
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d/commits", baseCtx.Username, baseCtx.Reponame, pr.Index))
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		// Get first commit URL
		commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, commitURL)

		commitID := path.Base(commitURL)

		addCommitStatus := func(status api.CommitStatusState) func(*testing.T) {
			return doAPICreateCommitStatus(ctx, commitID, api.CreateStatusOption{
				State:       status,
				TargetURL:   "http://test.ci/",
				Description: "",
				Context:     "testci",
			})
		}

		// Call API to add Pending status for commit
		t.Run("CreateStatus", addCommitStatus(api.CommitStatusPending))

		// Cancel not existing auto merge
		ctx.ExpectedCode = http.StatusNotFound
		t.Run("CancelAutoMergePR", doAPICancelAutoMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))

		// Add auto merge request
		ctx.ExpectedCode = http.StatusCreated
		t.Run("AutoMergePR", doAPIAutoMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))

		// Can not create schedule twice
		ctx.ExpectedCode = http.StatusConflict
		t.Run("AutoMergePRTwice", doAPIAutoMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))

		// Cancel auto merge request
		ctx.ExpectedCode = http.StatusNoContent
		t.Run("CancelAutoMergePR", doAPICancelAutoMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))

		// Add auto merge request
		ctx.ExpectedCode = http.StatusCreated
		t.Run("AutoMergePR", doAPIAutoMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))

		// Check pr status
		ctx.ExpectedCode = 0
		pr, err = doAPIGetPullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index)(t)
		assert.NoError(t, err)
		assert.False(t, pr.HasMerged)

		// Call API to add Failure status for commit
		t.Run("CreateStatus", addCommitStatus(api.CommitStatusFailure))

		// Check pr status
		pr, err = doAPIGetPullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index)(t)
		assert.NoError(t, err)
		assert.False(t, pr.HasMerged)

		// Call API to add Success status for commit
		t.Run("CreateStatus", addCommitStatus(api.CommitStatusSuccess))

		// wait to let gitea merge stuff
		time.Sleep(time.Second)

		// test pr status
		pr, err = doAPIGetPullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index)(t)
		assert.NoError(t, err)
		assert.True(t, pr.HasMerged)
	}
}

func doCreateAgitFlowPull(dstPath string, ctx *APITestContext, headBranch string) func(t *testing.T) {
	return func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// skip this test if git version is low
		if !git.DefaultFeatures().SupportProcReceive {
			return
		}

		gitRepo, err := git.OpenRepository(git.DefaultContext, dstPath)
		require.NoError(t, err)

		defer gitRepo.Close()

		var (
			pr1, pr2 *issues_model.PullRequest
			commit   string
		)
		repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, ctx.Username, ctx.Reponame)
		require.NoError(t, err)

		pullNum := unittest.GetCount(t, &issues_model.PullRequest{})

		t.Run("CreateHeadBranch", doGitCreateBranch(dstPath, headBranch))

		t.Run("AddCommit", func(t *testing.T) {
			err := os.WriteFile(path.Join(dstPath, "test_file"), []byte("## test content"), 0o666)
			require.NoError(t, err)

			err = git.AddChanges(dstPath, true)
			assert.NoError(t, err)

			err = git.CommitChanges(dstPath, git.CommitChangesOptions{
				Committer: &git.Signature{
					Email: "user2@example.com",
					Name:  "user2",
					When:  time.Now(),
				},
				Author: &git.Signature{
					Email: "user2@example.com",
					Name:  "user2",
					When:  time.Now(),
				},
				Message: "Testing commit 1",
			})
			assert.NoError(t, err)
			commit, err = gitRepo.GetRefCommitID("HEAD")
			assert.NoError(t, err)
		})

		t.Run("Push", func(t *testing.T) {
			err := git.NewCommand("push", "origin", "HEAD:refs/for/master", "-o").AddDynamicArguments("topic="+headBranch).Run(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			unittest.AssertCount(t, &issues_model.PullRequest{}, pullNum+1)
			pr1 = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
				HeadRepoID: repo.ID,
				Flow:       issues_model.PullRequestFlowAGit,
			})
			require.NotEmpty(t, pr1)

			prMsg, err := doAPIGetPullRequest(*ctx, ctx.Username, ctx.Reponame, pr1.Index)(t)
			require.NoError(t, err)

			assert.Equal(t, "user2/"+headBranch, pr1.HeadBranch)
			assert.False(t, prMsg.HasMerged)
			assert.Contains(t, "Testing commit 1", prMsg.Body)
			assert.Equal(t, commit, prMsg.Head.Sha)

			_, _, err = git.NewCommand("push", "origin").AddDynamicArguments("HEAD:refs/for/master/test/"+headBranch).RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			unittest.AssertCount(t, &issues_model.PullRequest{}, pullNum+2)
			pr2 = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
				HeadRepoID: repo.ID,
				Index:      pr1.Index + 1,
				Flow:       issues_model.PullRequestFlowAGit,
			})
			require.NotEmpty(t, pr2)

			prMsg, err = doAPIGetPullRequest(*ctx, ctx.Username, ctx.Reponame, pr2.Index)(t)
			require.NoError(t, err)

			assert.Equal(t, "user2/test/"+headBranch, pr2.HeadBranch)
			assert.False(t, prMsg.HasMerged)
		})

		if pr1 == nil || pr2 == nil {
			return
		}

		t.Run("AddCommit2", func(t *testing.T) {
			err := os.WriteFile(path.Join(dstPath, "test_file"), []byte("## test content \n ## test content 2"), 0o666)
			require.NoError(t, err)

			err = git.AddChanges(dstPath, true)
			assert.NoError(t, err)

			err = git.CommitChanges(dstPath, git.CommitChangesOptions{
				Committer: &git.Signature{
					Email: "user2@example.com",
					Name:  "user2",
					When:  time.Now(),
				},
				Author: &git.Signature{
					Email: "user2@example.com",
					Name:  "user2",
					When:  time.Now(),
				},
				Message: "Testing commit 2",
			})
			assert.NoError(t, err)
			commit, err = gitRepo.GetRefCommitID("HEAD")
			assert.NoError(t, err)
		})

		t.Run("Push2", func(t *testing.T) {
			err := git.NewCommand("push", "origin", "HEAD:refs/for/master", "-o").AddDynamicArguments("topic="+headBranch).Run(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			unittest.AssertCount(t, &issues_model.PullRequest{}, pullNum+2)
			prMsg, err := doAPIGetPullRequest(*ctx, ctx.Username, ctx.Reponame, pr1.Index)(t)
			require.NoError(t, err)

			assert.False(t, prMsg.HasMerged)
			assert.Equal(t, commit, prMsg.Head.Sha)

			_, _, err = git.NewCommand("push", "origin").AddDynamicArguments("HEAD:refs/for/master/test/"+headBranch).RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			unittest.AssertCount(t, &issues_model.PullRequest{}, pullNum+2)
			prMsg, err = doAPIGetPullRequest(*ctx, ctx.Username, ctx.Reponame, pr2.Index)(t)
			require.NoError(t, err)

			assert.False(t, prMsg.HasMerged)
			assert.Equal(t, commit, prMsg.Head.Sha)
		})
		t.Run("Merge", doAPIMergePullRequest(*ctx, ctx.Username, ctx.Reponame, pr1.Index))
		t.Run("CheckoutMasterAgain", doGitCheckoutBranch(dstPath, "master"))
	}
}
