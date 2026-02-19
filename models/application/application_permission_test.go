package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppPermList_ValidOrDefault(t *testing.T) {
	result := AppPermList("repository.contents:read,repository.issues:write,organization.members:none,unknown.unknown:unknow").ValidOrDefault()
	assert.Equal(t, AppPermList("repository.contents:read,repository.issues:write"), result)

	result = AppPermList("").ValidOrDefault()
	assert.Equal(t, AppPermList(""), result)

	result = AppPermList("xxx:yyy:zzz").ValidOrDefault()
	assert.Equal(t, AppPermList(""), result)
}
