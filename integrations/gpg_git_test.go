// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

func TestGPGGit(t *testing.T) {
	username := "user2"

	// OK Set a new GPG home
	tmpDir, err := ioutil.TempDir("", "temp-gpg")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = os.Chmod(tmpDir, 0700)
	assert.NoError(t, err)

	oldGNUPGHome := os.Getenv("GNUPGHOME")
	err = os.Setenv("GNUPGHOME", tmpDir)
	assert.NoError(t, err)
	defer os.Setenv("GNUPGHOME", oldGNUPGHome)

	// Need to create a root key
	rootKeyPair, err := createGPGKey(tmpDir, "gitea", "gitea@fake.local")
	assert.NoError(t, err)

	rootKeyID := rootKeyPair.PrimaryKey.KeyIdShortString()

	oldKeyID := setting.Repository.Signing.SigningKey
	oldName := setting.Repository.Signing.SigningName
	oldEmail := setting.Repository.Signing.SigningEmail
	defer func() {
		setting.Repository.Signing.SigningKey = oldKeyID
		setting.Repository.Signing.SigningName = oldName
		setting.Repository.Signing.SigningEmail = oldEmail
	}()

	setting.Repository.Signing.SigningKey = rootKeyID
	setting.Repository.Signing.SigningName = "gitea"
	setting.Repository.Signing.SigningEmail = "gitea@fake.local"
	user := models.AssertExistsAndLoadBean(t, &models.User{Name: username}).(*models.User)

	setting.Repository.Signing.InitialCommit = []string{"never"}
	setting.Repository.Signing.CRUDActions = []string{"never"}

	baseAPITestContext := NewAPITestContext(t, username, "repo1")
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("Unsigned-Initial", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
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
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"parentsigned"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("Unsigned-Initial-CRUD-ParentSigned", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			t.Run("CreateCRUDFile-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "master", "parentsigned", "signed-parent.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
			t.Run("CreateCRUDFile-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "parentsigned", "parentsigned2", "signed-parent2.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"never"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("Unsigned-Initial-CRUD-Never", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			t.Run("CreateCRUDFile-Never", crudActionCreateFile(
				t, testCtx, user, "parentsigned", "parentsigned-never", "unsigned-never2.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"always"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("Unsigned-Initial-CRUD-Always", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			t.Run("CreateCRUDFile-Always", crudActionCreateFile(
				t, testCtx, user, "master", "always", "signed-always.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
			t.Run("CreateCRUDFile-ParentSigned-always", crudActionCreateFile(
				t, testCtx, user, "parentsigned", "parentsigned-always", "signed-parent2.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"parentsigned"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("Unsigned-Initial-CRUD-ParentSigned", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			t.Run("CreateCRUDFile-Always-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "always", "always-parentsigned", "signed-always-parentsigned.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})
	}, false)
	setting.Repository.Signing.InitialCommit = []string{"always"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("AlwaysSign-Initial", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always")
			t.Run("CreateRepository", doAPICreateRepository(testCtx, false))
			t.Run("CheckMasterBranchSigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.True(t, branch.Commit.Verification.Verified)
				assert.Equal(t, "gitea@fake.local", branch.Commit.Verification.Signer.Email)
			}))
		})
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"never"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("AlwaysSign-Initial-CRUD-Never", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always")
			t.Run("CreateCRUDFile-Never", crudActionCreateFile(
				t, testCtx, user, "master", "never", "unsigned-never.txt", func(t *testing.T, response api.FileResponse) {
					assert.False(t, response.Verification.Verified)
				}))
		})
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"parentsigned"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("AlwaysSign-Initial-CRUD-ParentSigned-On-Always", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always")
			t.Run("CreateCRUDFile-ParentSigned", crudActionCreateFile(
				t, testCtx, user, "master", "parentsigned", "signed-parent.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))
		})
	}, false)
	setting.Repository.Signing.CRUDActions = []string{"always"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("AlwaysSign-Initial-CRUD-Always", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-always")
			t.Run("CreateCRUDFile-Always", crudActionCreateFile(
				t, testCtx, user, "master", "always", "signed-always.txt", func(t *testing.T, response api.FileResponse) {
					assert.True(t, response.Verification.Verified)
					assert.Equal(t, "gitea@fake.local", response.Verification.Signer.Email)
				}))

		})
	}, false)
	var pr api.PullRequest
	setting.Repository.Signing.Merges = []string{"commitssigned"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("UnsignedMerging", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			var err error
			t.Run("CreatePullRequest", func(t *testing.T) {
				pr, err = doAPICreatePullRequest(testCtx, testCtx.Username, testCtx.Reponame, "master", "never2")(t)
				assert.NoError(t, err)
			})
			t.Run("MergePR", doAPIMergePullRequest(testCtx, testCtx.Username, testCtx.Reponame, pr.Index))
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.False(t, branch.Commit.Verification.Verified)
				assert.Empty(t, branch.Commit.Verification.Signature)
			}))
		})
	}, false)
	setting.Repository.Signing.Merges = []string{"basesigned"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("BaseSignedMerging", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			var err error
			t.Run("CreatePullRequest", func(t *testing.T) {
				pr, err = doAPICreatePullRequest(testCtx, testCtx.Username, testCtx.Reponame, "master", "parentsigned2")(t)
				assert.NoError(t, err)
			})
			t.Run("MergePR", doAPIMergePullRequest(testCtx, testCtx.Username, testCtx.Reponame, pr.Index))
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.False(t, branch.Commit.Verification.Verified)
				assert.Empty(t, branch.Commit.Verification.Signature)
			}))
		})
	}, false)
	setting.Repository.Signing.Merges = []string{"commitssigned"}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = baseAPITestContext.GitPath()

		t.Run("CommitsSignedMerging", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			testCtx := NewAPITestContext(t, username, "initial-unsigned")
			var err error
			t.Run("CreatePullRequest", func(t *testing.T) {
				pr, err = doAPICreatePullRequest(testCtx, testCtx.Username, testCtx.Reponame, "master", "always-parentsigned")(t)
				assert.NoError(t, err)
			})
			t.Run("MergePR", doAPIMergePullRequest(testCtx, testCtx.Username, testCtx.Reponame, pr.Index))
			t.Run("CheckMasterBranchUnsigned", doAPIGetBranch(testCtx, "master", func(t *testing.T, branch api.Branch) {
				assert.NotNil(t, branch.Commit)
				assert.NotNil(t, branch.Commit.Verification)
				assert.True(t, branch.Commit.Verification.Verified)
			}))

		})
	}, false)
}

func crudActionCreateFile(t *testing.T, ctx APITestContext, user *models.User, from, to, path string, callback ...func(*testing.T, api.FileResponse)) func(*testing.T) {
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
		Content: base64.StdEncoding.EncodeToString([]byte("This is new text")),
	}, callback...)
}

func createGPGKey(tmpDir, name, email string) (*openpgp.Entity, error) {
	keyPair, err := openpgp.NewEntity(name, "test", email, nil)
	if err != nil {
		return nil, err
	}

	for _, id := range keyPair.Identities {
		err := id.SelfSignature.SignUserId(id.UserId.Id, keyPair.PrimaryKey, keyPair.PrivateKey, nil)
		if err != nil {
			return nil, err
		}
	}

	keyFile := filepath.Join(tmpDir, "temporary.key")
	keyWriter, err := os.Create(keyFile)
	if err != nil {
		return nil, err
	}
	defer keyWriter.Close()
	defer os.Remove(keyFile)

	w, err := armor.Encode(keyWriter, openpgp.PrivateKeyType, nil)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	keyPair.SerializePrivate(w, nil)
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := keyWriter.Close(); err != nil {
		return nil, err
	}

	if _, _, err := process.GetManager().Exec("gpg --import temporary.key", "gpg", "--import", keyFile); err != nil {
		return nil, err
	}
	return keyPair, nil
}
