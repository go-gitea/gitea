package repo_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/testutil"
	api "github.com/go-gitea/go-sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	testutil.TestGlobalInit()

	os.Exit(m.Run())
}

func TestIssueIndex(t *testing.T) {
	testutil.PrepareTestDatabase()

	w, r := testutil.NewTestContext("GET", "/api/v1/repos/user1/foo/issues", "", nil, "1")
	testutil.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIssueShow(t *testing.T) {
	testutil.PrepareTestDatabase()

	w, r := testutil.NewTestContext("GET", "/api/v1/repos/user1/foo/issues/1", "", nil, "1")
	testutil.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	issue := new(api.Issue)
	err := json.Unmarshal(w.Body.Bytes(), &issue)
	assert.NoError(t, err)
	assert.Equal(t, "Title", issue.Title)
	assert.Equal(t, "Content", issue.Body)
	assert.Equal(t, "user1", issue.Poster.UserName)
}

func TestCreate(t *testing.T) {
	testutil.PrepareTestDatabase()

	bytes, _ := json.Marshal(api.Issue{
		Title: "A issue title",
		Body:  "Please fix",
	})
	count := testutil.TableCount("issue")
	w, r := testutil.NewTestContext("POST", "/api/v1/repos/user1/foo/issues", testutil.CONTENT_TYPE_JSON, bytes, "1")
	testutil.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, count+1, testutil.TableCount("issue"))
	issue, _ := models.GetIssueByID(testutil.LastId("issue"))
	assert.Equal(t, "A issue title", issue.Title)
	assert.Equal(t, "Please fix", issue.Content)
}

func TestEdit(t *testing.T) {
	testutil.PrepareTestDatabase()

	bytes, _ := json.Marshal(api.Issue{
		Title: "Edited title",
		Body:  "Edited content",
	})
	w, r := testutil.NewTestContext("PATCH", "/api/v1/repos/user1/foo/issues/1", testutil.CONTENT_TYPE_JSON, bytes, "1")
	testutil.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
	issue, _ := models.GetIssueByID(1)
	assert.Equal(t, "Edited title", issue.Title)
	assert.Equal(t, "Edited content", issue.Content)
}
