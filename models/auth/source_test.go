// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"strings"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

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
	auth_model.UnmarshalAuthSourceConfig(bs, source)
	return nil
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

func TestUnmarshalAuthSourceConfig(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.IsProd, true)()
	defer test.MockVariableValue(&setting.IsInTesting, false)()

	cfg := &TestSource{}
	source := &auth_model.Source{Type: auth_model.OAuth2, Name: t.Name(), IsActive: true, Cfg: cfg}
	err := db.Insert(t.Context(), source)
	require.NoError(t, err)
	cfg.SetAuthSource(source)

	t.Run("ValidConfig", func(t *testing.T) {
		for _, data := range []string{`{"TestField":"TestValue"}`, "", "null"} {
			auth_model.UnmarshalAuthSourceConfig([]byte(data), cfg)
			assert.Equal(t, "TestValue", cfg.TestField)
			assert.Same(t, source, cfg.GetAuthSource())
		}
	})

	t.Run("InvalidConfigDeactivatesSource", func(t *testing.T) {
		_, err = db.GetEngine(t.Context()).ID(source.ID).NoAutoTime().Cols("updated_unix").Update(&auth_model.Source{UpdatedUnix: timeutil.TimeStampNow() - 60})
		require.NoError(t, err)
		has, _ := db.GetEngine(t.Context()).ID(source.ID).And("is_active=?", true).Get(&auth_model.Source{})
		assert.True(t, has)

		auth_model.UnmarshalAuthSourceConfig([]byte(`{"TestField":false}`), cfg)
		assert.Same(t, source, cfg.GetAuthSource())
		require.Eventually(t, func() bool {
			has, _ := db.GetEngine(t.Context()).ID(source.ID).And("is_active=?", false).Get(&auth_model.Source{})
			return has
		}, time.Second, time.Millisecond)
	})
}
