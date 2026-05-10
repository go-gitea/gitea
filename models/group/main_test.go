package group_test

import (
	"testing"

	_ "code.gitea.io/gitea/models/group"

	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
