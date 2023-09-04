package issues_test

import (
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	// 写个表格测试

	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}), // repo 1
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}), // repo 1
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 5}), // repo 1
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 7}), // repo 2
	}

	iws, err := issues_model.GetIssueSubscribers(db.DefaultContext, issueList[0], db.ListOptions{})
	assert.NoError(t, err)
	// user 9 watch it ,but user status is inactive, thus 0
	// user 5 participates and is not explicitly not watching, thus 1
	// repo1 have 1,2,3 user watch it, thus 3
	// total 4
	assert.Len(t, iws, 4)

	iws, err = issues_model.GetIssueSubscribers(db.DefaultContext, issueList[1], db.ListOptions{})
	assert.NoError(t, err)
	// user[2] in Watcher is explicitly not watching, thus 0
	// user[1] participates and is not explicitly not watching, thus 1
	// repo1 have [1,2,3] user watch it, thus 2
	// total 3
	assert.Len(t, iws, 3)

	iws, err = issues_model.GetIssueSubscribers(db.DefaultContext, issueList[2], db.ListOptions{})
	assert.NoError(t, err)
	// Issue has no Watchers
	// repo[1] have [1,2,3] user watch it, thus 3
	// poster 2 participate, thus 1
	// total 4
	assert.Len(t, iws, 4)

	iws, err = issues_model.GetIssueSubscribers(db.DefaultContext, issueList[3], db.ListOptions{})
	assert.NoError(t, err)
	// Issue has one watcher, user 0
	// repo1 have no user watch it, thus 0
	// poster 1 participate, thus 	1
	assert.Len(t, iws, 1)
}
