package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsTokenPermissionsPersistence(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Create Repo
		apiRepo := createActionsTestRepo(t, token, "repo-persistence", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		defer doAPIDeleteRepository(NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository))(t)

		// 1. Enable Max Permissions
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", repo.OwnerName, repo.Name), map[string]string{
			"token_permission_mode":  "permissive",
			"enable_max_permissions": "true",
			"max_code":               "read",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Verify
		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		require.NoError(t, repo.LoadUnits(t.Context()))
		unit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
		require.NoError(t, err)
		cfg := unit.ActionsConfig()
		require.NotNil(t, cfg.MaxTokenPermissions, "MaxTokenPermissions should NOT be nil")
		assert.Equal(t, "read", cfg.MaxTokenPermissions.Code.ToString())

		// 2. Disable Max Permissions
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", repo.OwnerName, repo.Name), map[string]string{
			"token_permission_mode": "permissive",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Verify
		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		require.NoError(t, repo.LoadUnits(t.Context()))
		unit, err = repo.GetUnit(t.Context(), unit_model.TypeActions)
		require.NoError(t, err)
		cfg = unit.ActionsConfig()
		require.Nil(t, cfg.MaxTokenPermissions, "MaxTokenPermissions SHOULD be nil")
	})
}
