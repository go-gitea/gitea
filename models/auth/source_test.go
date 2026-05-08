// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type TestSource struct {
	auth_model.ConfigBase `json:"-"`

	TestField string
}

// FromDB fills up a LDAPConfig from serialized format.
func (source *TestSource) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &source)
}

// ToDB exports a LDAPConfig to a serialized format.
func (source *TestSource) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

func TestDumpAuthSource(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	authSourceSchema, err := unittest.GetXORMEngine().TableInfo(new(auth_model.Source))
	require.NoError(t, err)

	auth_model.RegisterTypeConfig(auth_model.OAuth2, new(TestSource))
	source := &auth_model.Source{
		Type: auth_model.OAuth2,
		Name: "TestSource",
		Cfg:  &TestSource{TestField: "TestValue"},
	}
	require.NoError(t, auth_model.CreateSource(t.Context(), source))

	// intentionally test the "dump" to make sure the dumped JSON is correct: https://github.com/go-gitea/gitea/pull/16847
	sb := &strings.Builder{}
	require.NoError(t, unittest.GetXORMEngine().DumpTables([]*schemas.Table{authSourceSchema}, sb))
	// the dumped SQL is something like:
	// INSERT INTO `login_source` (`id`, `type`, `name`, `is_active`, `is_sync_enabled`, `two_factor_policy`, `cfg`, `created_unix`, `updated_unix`) VALUES (1,6,'TestSource',0,0,'','{"TestField":"TestValue"}',1774179784,1774179784);
	assert.Contains(t, sb.String(), `'{"TestField":"TestValue"}'`)
}
