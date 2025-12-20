package integration

import (
	"net/http"
	"testing"

	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func TestCreateGroup(t *testing.T) {

}

func TestOwnersAndAdminsCanSeeAllTopLevelGroups(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestf(t, "GET", "/api/v1/orgs/org-with-groups/groups").AddBasicAuth("user2")
	resp := MakeRequest(t, req, http.StatusOK)
	groups := make([]*api.Group, 0)
	json.NewDecoder(resp.Body).Decode(&groups)
	expectedLen := unittest.GetCount(t, new(group_model.Group),
		group_model.FindGroupsOptions{
			ParentGroupID: 0,
			OwnerID:       43,
		}.ToConds())
	assert.Len(t, groups, expectedLen)

	// now test if site-wide admin can see all groups
	req = NewRequestf(t, "GET", "/api/v1/orgs/org-with-groups/groups").AddBasicAuth("user1")
	resp = MakeRequest(t, req, http.StatusOK)
	groups = make([]*api.Group, 0)
	json.NewDecoder(resp.Body).Decode(&groups)
	assert.Len(t, groups, expectedLen)
}

func TestNonOrgMemberWontSeeHiddenTopLevelGroups(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestf(t, "GET", "/api/v1/orgs/org-with-groups/groups").AddBasicAuth("user4")
	resp := MakeRequest(t, req, http.StatusOK)
	groups := make([]*api.Group, 0)
	json.NewDecoder(resp.Body).Decode(&groups)
	expectedLen := unittest.GetCount(t, new(group_model.Group),
		group_model.FindGroupsOptions{
			ParentGroupID: 0,
			OwnerID:       43,
		}.ToConds())
	assert.NotEqual(t, len(groups), expectedLen)
}

func TestGroupNotAccessibleWhenParentIsPrivate(t *testing.T) {

}
