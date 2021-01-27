// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestViewDeployKeysNoLogin(t *testing.T) {
	defer prepareTestEnv(t)()
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/keys")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateDeployKeyNoLogin(t *testing.T) {
	defer prepareTestEnv(t)()
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/keys", api.CreateKeyOption{
		Title: "title",
		Key:   "key",
	})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestGetDeployKeyNoLogin(t *testing.T) {
	defer prepareTestEnv(t)()
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestDeleteDeployKeyNoLogin(t *testing.T) {
	defer prepareTestEnv(t)()
	req := NewRequest(t, "DELETE", "/api/v1/repos/user2/repo1/keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateReadOnlyDeployKey(t *testing.T) {
	defer prepareTestEnv(t)()
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{Name: "repo1"}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/keys?token=%s", repoOwner.Name, repo.Name, token)
	rawKeyBody := api.CreateKeyOption{
		Title:    "read-only",
		Key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		ReadOnly: true,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, rawKeyBody)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newDeployKey api.DeployKey
	DecodeJSON(t, resp, &newDeployKey)
	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		ID:      newDeployKey.ID,
		Name:    rawKeyBody.Title,
		Content: rawKeyBody.Key,
		Mode:    models.AccessModeRead,
	})
}

func TestCreateReadWriteDeployKey(t *testing.T) {
	defer prepareTestEnv(t)()
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{Name: "repo1"}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/keys?token=%s", repoOwner.Name, repo.Name, token)
	rawKeyBody := api.CreateKeyOption{
		Title: "read-write",
		Key:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
	}
	req := NewRequestWithJSON(t, "POST", keysURL, rawKeyBody)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newDeployKey api.DeployKey
	DecodeJSON(t, resp, &newDeployKey)
	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		ID:      newDeployKey.ID,
		Name:    rawKeyBody.Title,
		Content: rawKeyBody.Key,
		Mode:    models.AccessModeWrite,
	})
}

func TestCreateUserKey(t *testing.T) {
	defer prepareTestEnv(t)()
	user := models.AssertExistsAndLoadBean(t, &models.User{Name: "user1"}).(*models.User)

	session := loginUser(t, "user1")
	token := url.QueryEscape(getTokenForLoggedInUser(t, session))
	keysURL := fmt.Sprintf("/api/v1/user/keys?token=%s", token)
	keyType := "ssh-rsa"
	keyContent := "AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM="
	rawKeyBody := api.CreateKeyOption{
		Title: "test-key",
		Key:   keyType + " " + keyContent,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, rawKeyBody)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)
	models.AssertExistsAndLoadBean(t, &models.PublicKey{
		ID:      newPublicKey.ID,
		OwnerID: user.ID,
		Name:    rawKeyBody.Title,
		Content: rawKeyBody.Key,
		Mode:    models.AccessModeWrite,
	})

	// Search by fingerprint
	fingerprintURL := fmt.Sprintf("/api/v1/user/keys?token=%s&fingerprint=%s", token, newPublicKey.Fingerprint)

	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	var fingerprintPublicKeys []api.PublicKey
	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Equal(t, newPublicKey.Fingerprint, fingerprintPublicKeys[0].Fingerprint)
	assert.Equal(t, newPublicKey.ID, fingerprintPublicKeys[0].ID)
	assert.Equal(t, user.ID, fingerprintPublicKeys[0].Owner.ID)

	fingerprintURL = fmt.Sprintf("/api/v1/users/%s/keys?token=%s&fingerprint=%s", user.Name, token, newPublicKey.Fingerprint)

	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Equal(t, newPublicKey.Fingerprint, fingerprintPublicKeys[0].Fingerprint)
	assert.Equal(t, newPublicKey.ID, fingerprintPublicKeys[0].ID)
	assert.Equal(t, user.ID, fingerprintPublicKeys[0].Owner.ID)

	// Fail search by fingerprint
	fingerprintURL = fmt.Sprintf("/api/v1/user/keys?token=%s&fingerprint=%sA", token, newPublicKey.Fingerprint)

	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Len(t, fingerprintPublicKeys, 0)

	// Fail searching for wrong users key
	fingerprintURL = fmt.Sprintf("/api/v1/users/%s/keys?token=%s&fingerprint=%s", "user2", token, newPublicKey.Fingerprint)
	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Len(t, fingerprintPublicKeys, 0)

	// Now login as user 2
	session2 := loginUser(t, "user2")
	token2 := url.QueryEscape(getTokenForLoggedInUser(t, session2))

	// Should find key even though not ours, but we shouldn't know whose it is
	fingerprintURL = fmt.Sprintf("/api/v1/user/keys?token=%s&fingerprint=%s", token2, newPublicKey.Fingerprint)
	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Equal(t, newPublicKey.Fingerprint, fingerprintPublicKeys[0].Fingerprint)
	assert.Equal(t, newPublicKey.ID, fingerprintPublicKeys[0].ID)
	assert.Nil(t, fingerprintPublicKeys[0].Owner)

	// Should find key even though not ours, but we shouldn't know whose it is
	fingerprintURL = fmt.Sprintf("/api/v1/users/%s/keys?token=%s&fingerprint=%s", user.Name, token2, newPublicKey.Fingerprint)

	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Equal(t, newPublicKey.Fingerprint, fingerprintPublicKeys[0].Fingerprint)
	assert.Equal(t, newPublicKey.ID, fingerprintPublicKeys[0].ID)
	assert.Nil(t, fingerprintPublicKeys[0].Owner)

	// Fail when searching for key if it is not ours
	fingerprintURL = fmt.Sprintf("/api/v1/users/%s/keys?token=%s&fingerprint=%s", "user2", token2, newPublicKey.Fingerprint)
	req = NewRequest(t, "GET", fingerprintURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &fingerprintPublicKeys)
	assert.Len(t, fingerprintPublicKeys, 0)
}
