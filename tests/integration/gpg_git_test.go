// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/stretchr/testify/assert"
)

func TestGPGGit(t *testing.T) {
	tmpDir := t.TempDir() // use a temp dir to avoid messing with the user's GPG keyring
	err := os.Chmod(tmpDir, 0o700)
	assert.NoError(t, err)

	oldGNUPGHome := os.Getenv("GNUPGHOME")
	err = os.Setenv("GNUPGHOME", tmpDir)
	assert.NoError(t, err)
	defer os.Setenv("GNUPGHOME", oldGNUPGHome)

	// Need to create a root key
	rootKeyPair, err := importTestingKey()
	if !assert.NoError(t, err, "importTestingKey") {
		return
	}

	defer test.MockVariableValue(&setting.Repository.Signing.SigningKey, rootKeyPair.PrimaryKey.KeyIdShortString())()
	defer test.MockVariableValue(&setting.Repository.Signing.SigningName, "gitea")()
	defer test.MockVariableValue(&setting.Repository.Signing.SigningEmail, "gitea@fake.local")()
	defer test.MockVariableValue(&setting.Repository.Signing.InitialCommit, []string{"never"})()
	defer test.MockVariableValue(&setting.Repository.Signing.CRUDActions, []string{"never"})()

	username := "user2"
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: username})
	baseAPITestContext := NewAPITestContext(t, username, "repo1")

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("Unsigned-Initial", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateRepository", doAPICreateRepository(testCtx, false))
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.False(t, branch.Commit.Verification.Verified)
				assert.Empty(t, branch.Commit.Verification.Signature)
			}))
			t.Run("CreateCRUDFile-Never", crudActionCreateFile(
				t, testCtx, user, "master", "never", "unsigned-never.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
			t.Run("CreateCRUDFile-Never", crudActionCreateFile(
				t, testCtx, user, "never", "never2", "unsigned-never2.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})

		setting.Repository.Signing.CRUDActions = []string{"parentsigned"}
		t.Run("Unsigned-Initial-CRUD-ParentSigned", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateCRUDFile-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "master", "parentsigned", "signed-parent.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
			t.Run("CreateCRUDFile-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "parentsigned", "parentsigned2", "signed-parent2.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})

		setting.Repository.Signing.CRUDActions = []string{"never"}
		t.Run("Unsigned-Initial-CRUD-Never", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateCRUDFile-Never", crudActionCreateFile(
				t, testCtx, user, "parentsigned", "parentsigned-never", "unsigned-never2.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})

		setting.Repository.Signing.CRUDActions = []string{"always"}
		t.Run("Unsigned-Initial-CRUD-Always", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateCRUDFile-Always", crudActionCreateFile(
				t, testCtx, user, "master", "always", "signed-always.txt", func(t *testing.T, response api.FileResponse) {
					assert.NotNil(t, response.Verification)
					if response.Verification == nil {
						assert.FailNow(t, "no verification provided with response! %v", response)
					}
					assert.True(t, response.Verification.Verified)
					if !response.Verification.Verified {
						t.FailNow()
					}
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
			t.Run("CreateCRUDFile-ParentSigned-always", crudActionCreateFile(
				t, testCtx, user, "parentsigned", "parentsigned-always", "signed-parent2.txt", func(t *testing.T, response api.FileResponse) {
					assert.NotNil(t, response.Verification)
					if response.Verification == nil {
						assert.FailNow(t, "no verification provided with response! %v", response)
					}
					assert.True(t, response.Verification.Verified)
					if !response.Verification.Verified {
						t.FailNow()
					}
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})

		setting.Repository.Signing.CRUDActions = []string{"parentsigned"}
		t.Run("Unsigned-Initial-CRUD-ParentSigned", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateCRUDFile-Always-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "always", "always-parentsigned", "signed-always-parentsigned.txt", func(t *testing.T, response api.FileResponse) {
					assert.NotNil(t, response.Verification)
					if response.Verification == nil {
						assert.FailNow(t, "no verification provided with response! %v", response)
					}
					assert.True(t, response.Verification.Verified)
					if !response.Verification.Verified {
						t.FailNow()
					}
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})

		setting.Repository.Signing.InitialCommit = []string{"always"}
		t.Run("AlwaysSign-Initial", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateRepository", doAPICreateRepository(testCtx, false))
			t.Run("CheckMasterBranchSigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				if branch.Commit == nil {
					assert.FailNow(t, "no commit provided with branch! %v", branch)
				}
				assert.NotNil(t, branch.Commit.Verification)
				if branch.Commit.Verification == nil {
					assert.FailNow(t, "no verification provided with branch commit! %v", branch.Commit)
				}
				assert.True(t, branch.Commit.Verification.Verified)
				if !branch.Commit.Verification.Verified {
					t.FailNow()
				}
				assert.Equal(t, "gitea@fake.local", branch.Commit.Verification.Signer.Email)
			}))
		})

		setting.Repository.Signing.CRUDActions = []string{"never"}
		t.Run("AlwaysSign-Initial-CRUD-Never", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always-never", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateRepository", doAPICreateRepository(testCtx, false))
			t.Run("CreateCRUDFile-Never", crudActionCreateFile(
				t, testCtx, user, "master", "never", "unsigned-never.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})

		setting.Repository.Signing.CRUDActions = []string{"parentsigned"}
		t.Run("AlwaysSign-Initial-CRUD-ParentSigned-On-Always", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always-parent", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateRepository", doAPICreateRepository(testCtx, false))
			t.Run("CreateCRUDFile-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "master", "parentsigned", "signed-parent.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					if !response.Verification.Verified {
						t.FailNow()
						return
					}
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})

		setting.Repository.Signing.CRUDActions = []string{"always"}
		t.Run("AlwaysSign-Initial-CRUD-Always", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always-always", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreateRepository", doAPICreateRepository(testCtx, false))
			t.Run("CreateCRUDFile-Always", crudActionCreateFile(
				t, testCtx, user, "master", "always", "signed-always.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					if !response.Verification.Verified {
						t.FailNow()
						return
					}
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})

		setting.Repository.Signing.Merges = []string{"commitssigned"}
		t.Run("UnsignedMerging", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreatePullRequest", func(t *testing.T) {
				pr, err := doAPICreatePullRequest(testCtx, testCtx.Username, testCtx.Reponame, "master", "never2")(t)
				assert.NoError(t, err)
				t.Run("MergePR", doAPIMergePullRequest(testCtx, testCtx.Username, testCtx.Reponame, pr.Index))
			})
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.False(t, branch.Commit.Verification.Verified)
				assert.Empty(t, branch.Commit.Verification.Signature)
			}))
		})

		setting.Repository.Signing.Merges = []string{"basesigned"}
		t.Run("BaseSignedMerging", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreatePullRequest", func(t *testing.T) {
				pr, err := doAPICreatePullRequest(testCtx, testCtx.Username, testCtx.Reponame, "master", "parentsigned2")(t)
				assert.NoError(t, err)
				t.Run("MergePR", doAPIMergePullRequest(testCtx, testCtx.Username, testCtx.Reponame, pr.Index))
			})
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.False(t, branch.Commit.Verification.Verified)
				assert.Empty(t, branch.Commit.Verification.Signature)
			}))
		})

		setting.Repository.Signing.Merges = []string{"commitssigned"}
		t.Run("CommitsSignedMerging", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
			t.Run("CreatePullRequest", func(t *testing.T) {
				pr, err := doAPICreatePullRequest(testCtx, testCtx.Username, testCtx.Reponame, "master", "always-parentsigned")(t)
				assert.NoError(t, err)
				t.Run("MergePR", doAPIMergePullRequest(testCtx, testCtx.Username, testCtx.Reponame, pr.Index))
			})
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.True(t, branch.Commit.Verification.Verified)
			}))
		})
	})
}

func crudActionCreateFile(_ *testing.T, ctx APITestContext, user *user_model.User, from, to, path string, callback ...func(*testing.T, api.FileResponse)) func(*testing.T) {
	return doAPICreateFile(ctx, path, &api.CreateFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    from,
			NewBranchName: to,
			Message:       fmt.Sprintf("from:%s to:%s path:%s", from, to, path),
			Author: api.Identity{
				Name:  user.FullName,
				Email: user.Email,
			},
			Committer: api.Identity{
				Name:  user.FullName,
				Email: user.Email,
			},
		},
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("This is new text for %s", path))),
	}, callback...)
}

func importTestingKey() (*openpgp.Entity, error) {
	if _, _, err := process.GetManager().Exec("gpg --import tests/integration/private-testing.key", "gpg", "--import", "tests/integration/private-testing.key"); err != nil {
		return nil, err
	}
	keyringFile, err := os.Open("tests/integration/private-testing.key")
	if err != nil {
		return nil, err
	}
	defer keyringFile.Close()

	block, err := armor.Decode(keyringFile)
	if err != nil {
		return nil, err
	}

	keyring, err := openpgp.ReadKeyRing(block.Body)
	if err != nil {
		return nil, fmt.Errorf("Keyring access failed: '%w'", err)
	}

	// There should only be one entity in this file.
	return keyring[0], nil
}
