package integration

import (
	"code.gitea.io/gitea/tests"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestRepoMergeCommitRevert(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	commitToRevert := "deebcbc752e540bab4ce3ee713d3fc8fdc35b2f7"
	repoName := "test_commit_revert"

	req := NewRequest(t, "GET", "/user2/test_commit_revert/_cherrypick/deebcbc752e540bab4ce3ee713d3fc8fdc35b2f7/main?ref=main&refType=branch&cherry-pick-type=revert")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/test_commit_revert/_cherrypick/deebcbc752e540bab4ce3ee713d3fc8fdc35b2f7/main", repoName, commitToRevert), map[string]string{
		"_csrf":           htmlDoc.GetCSRF(),
		"last_commit":     commitToRevert,
		"page_has_posted": "true",
		"revert":          "true",
		"commit_summary":  "reverting test commit",
		"commit_message":  "test message",
		"commit_choice":   "direct",
		"new_branch_name": "test-revert-branch-1",
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	// A successful revert redirects to the main branch
	assert.EqualValues(t, "/user2/test_commit_revert/src/branch/main", resp.Header().Get("Location"))
}
