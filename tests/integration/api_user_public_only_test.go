// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func TestAPIPublicOnlySelfUserRoutes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user31"})
	require.True(t, privateUser.Visibility.IsPrivate())

	privateSession := loginUser(t, privateUser.Name)
	privateReadUserToken := getTokenForLoggedInUser(t, privateSession,
		auth_model.AccessTokenScopePublicOnly,
		auth_model.AccessTokenScopeReadUser,
	)
	privateWriteUserToken := getTokenForLoggedInUser(t, privateSession,
		auth_model.AccessTokenScopePublicOnly,
		auth_model.AccessTokenScopeWriteUser,
	)

	t.Run("PrivateProfileForbidden", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/users/user31").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
	})

	t.Run("PrivateSensitiveSelfRoutesForbidden", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/settings").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		hideEmail := true
		settingsReq := NewRequestWithJSON(t, "PATCH", "/api/v1/user/settings", &api.UserSettingsOptions{
			HideEmail: &hideEmail,
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, settingsReq, http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/emails").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		emailReq := NewRequestWithJSON(t, "POST", "/api/v1/user/emails", &api.CreateEmailOption{
			Emails: []string{"user31-public-only@example.com"},
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, emailReq, http.StatusForbidden)

		keyReq := NewRequestWithJSON(t, "POST", "/api/v1/user/keys", api.CreateKeyOption{
			Title: "public-only-private-key",
			Key:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment",
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, keyReq, http.StatusForbidden)

		oauthReq := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &api.CreateOAuth2ApplicationOptions{
			Name:               "public-only-private-oauth-app",
			RedirectURIs:       []string{"https://example.com/callback"},
			ConfidentialClient: true,
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, oauthReq, http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/gpg_keys").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		gpgKeyReq := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_keys", &api.CreateGPGKeyOption{
			ArmoredKey: "-----BEGIN PGP PUBLIC KEY BLOCK-----\ncomment\n-----END PGP PUBLIC KEY BLOCK-----",
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, gpgKeyReq, http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/gpg_key_token").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		gpgVerifyReq := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_key_verify", &api.VerifyGPGKeyOption{
			KeyID:     "deadbeef",
			Signature: "invalid-signature",
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, gpgVerifyReq, http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/actions/variables").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "DELETE", "/api/v1/user/actions/secrets/PRIVATE_SECRET").AddTokenAuth(privateWriteUserToken), http.StatusForbidden)
		variableReq := NewRequestWithJSON(t, "POST", "/api/v1/user/actions/variables/PRIVATE_VAR", api.CreateVariableOption{
			Value:       "private-value",
			Description: "must stay private",
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, variableReq, http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "POST", "/api/v1/user/actions/runners/registration-token").AddTokenAuth(privateWriteUserToken), http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/hooks").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		hookReq := NewRequestWithJSON(t, "POST", "/api/v1/user/hooks", api.CreateHookOption{
			Type: "gitea",
			Config: api.CreateHookOptionConfig{
				"content_type": "json",
				"url":          "http://example.com/",
			},
			Name: "public-only-private-hook",
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, hookReq, http.StatusForbidden)

		avatarReq := NewRequestWithJSON(t, "POST", "/api/v1/user/avatar", &api.UpdateUserAvatarOption{
			Image: "aGVsbG8=",
		}).AddTokenAuth(privateWriteUserToken)
		MakeRequest(t, avatarReq, http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "DELETE", "/api/v1/user/avatar").AddTokenAuth(privateWriteUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/times").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/stopwatches").AddTokenAuth(privateReadUserToken), http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/subscriptions").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/teams").AddTokenAuth(privateReadUserToken), http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/blocks").AddTokenAuth(privateReadUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "PUT", "/api/v1/user/blocks/user2").AddTokenAuth(privateWriteUserToken), http.StatusForbidden)

		MakeRequest(t, NewRequest(t, "PUT", "/api/v1/user/following/user2").AddTokenAuth(privateWriteUserToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "DELETE", "/api/v1/user/following/user2").AddTokenAuth(privateWriteUserToken), http.StatusForbidden)
	})

	t.Run("PublicRepoRoutesFilterAndRejectMutations", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		publicSession := loginUser(t, "user2")
		fullWriteRepoToken := getTokenForLoggedInUser(t, publicSession,
			auth_model.AccessTokenScopeWriteUser,
			auth_model.AccessTokenScopeWriteRepository,
		)
		publicOnlyReadRepoToken := getTokenForLoggedInUser(t, publicSession,
			auth_model.AccessTokenScopePublicOnly,
			auth_model.AccessTokenScopeReadUser,
			auth_model.AccessTokenScopeReadRepository,
		)
		publicOnlyWriteRepoToken := getTokenForLoggedInUser(t, publicSession,
			auth_model.AccessTokenScopePublicOnly,
			auth_model.AccessTokenScopeWriteUser,
			auth_model.AccessTokenScopeWriteRepository,
		)

		publicRepoName := "public-only-visible-self-repo"
		privateRepoName := "public-only-hidden-self-repo"

		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
			Name:    publicRepoName,
			Private: false,
		}).AddTokenAuth(fullWriteRepoToken), http.StatusCreated)
		publicRepo := DecodeJSON(t, resp, &api.Repository{})
		require.Equal(t, "user2/"+publicRepoName, publicRepo.FullName)

		resp = MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
			Name:    privateRepoName,
			Private: true,
		}).AddTokenAuth(fullWriteRepoToken), http.StatusCreated)
		privateRepo := DecodeJSON(t, resp, &api.Repository{})
		require.Equal(t, "user2/"+privateRepoName, privateRepo.FullName)

		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/"+privateRepoName).AddTokenAuth(publicOnlyReadRepoToken), http.StatusNotFound)

		resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/repos").AddTokenAuth(publicOnlyReadRepoToken), http.StatusOK)
		repos := DecodeJSON(t, resp, []api.Repository{})

		foundPublicRepo := false
		for _, repo := range repos {
			require.NotEqual(t, privateRepo.FullName, repo.FullName)
			if repo.FullName == publicRepo.FullName {
				foundPublicRepo = true
			}
		}
		require.True(t, foundPublicRepo)

		MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
			Name:    "public-only-rejected-self-repo",
			Private: false,
		}).AddTokenAuth(publicOnlyWriteRepoToken), http.StatusForbidden)
	})
}
