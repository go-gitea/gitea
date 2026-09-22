package egress

import (
	"testing"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2AvatarAllowListRestricts(t *testing.T) {
	defer test.MockVariableValue(&setting.Security.AllowedHostList, "avatars.example.com")()
	avatarPolicy := newOauth2AvatarPolicy()
	assert.True(t, avatarPolicy.AllowsHost("avatars.example.com"), "the configured host must be allowed")
	assert.False(t, avatarPolicy.AllowsHost("8.8.8.8"), "an unrelated external host must be rejected")

	// the default `external` allow-list still permits external hosts
	setting.Security.AllowedHostList = policy.MatchBuiltinExternal
	assert.True(t, newOauth2AvatarPolicy().AllowsHost("8.8.8.8"), "default allow-list permits external hosts")
}
