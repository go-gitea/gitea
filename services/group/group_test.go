package group

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

// group 12 is private
// team 23 are owners

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestNewGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const groupName = "group x"
	group := &group_model.Group{
		Name:    groupName,
		OwnerID: 3,
	}
	assert.NoError(t, NewGroup(db.DefaultContext, group))
	unittest.AssertExistsAndLoadBean(t, &group_model.Group{Name: groupName})
}

func TestMoveGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testfn := func(gid int64) {
		cond := &group_model.FindGroupsOptions{
			ParentGroupID: 123,
			OwnerID:       3,
		}
		origCount := unittest.GetCount(t, new(group_model.Group), cond.ToConds())

		assert.NoError(t, MoveGroupItem(t.Context(), gid, 123, true, -1))
		unittest.AssertCountByCond(t, "repo_group", cond.ToConds(), origCount+1)
	}
	testfn(124)
	testfn(132)
	testfn(150)
}

func TestMoveRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	cond := repo_model.SearchRepositoryCondition(repo_model.SearchRepoOptions{
		GroupID: 123,
	})
	origCount := unittest.GetCount(t, new(repo_model.Repository), cond)

	assert.NoError(t, MoveGroupItem(db.DefaultContext, 32, 123, false, -1))
	unittest.AssertCountByCond(t, "repository", cond, origCount+1)
}
