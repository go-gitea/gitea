package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertDeleteBranch(t *testing.T, session *TestSession, repoPath, branchName string, success bool) {
	req, err := http.NewRequest("POST", fmt.Sprintf("/%s/branches/%s/delete", repoPath, branchName), nil)
	assert.NoError(t, err)
	resp := session.MakeRequest(t, req)
	if success {
		assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	} else {
		assert.EqualValues(t, 400, resp.HeaderCode)
	}
}

func TestDeleteBranches(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")
	assertDeleteBranch(t, session, "user2/repo1", "master", false)
	assertDeleteBranch(t, session, "user2/repo1", "dev", false)
	assertDeleteBranch(t, session, "user2/repo1", "lunny/test", false)
}
