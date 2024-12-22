// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type TestSource struct {
	Provider                      string
	ClientID                      string
	ClientSecret                  string
	OpenIDConnectAutoDiscoveryURL string
	IconURL                       string
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
	assert.NoError(t, unittest.PrepareTestDatabase())

	authSourceSchema, err := db.TableInfo(new(auth_model.Source))
	assert.NoError(t, err)

	auth_model.RegisterTypeConfig(auth_model.OAuth2, new(TestSource))

	auth_model.CreateSource(db.DefaultContext, &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     "TestSource",
		IsActive: false,
		Cfg: &TestSource{
			Provider: "ConvertibleSourceName",
			ClientID: "42",
		},
	})

	sb := new(strings.Builder)

	// TODO: this test is quite hacky, it should use a low-level "select" (without model processors) but not a database dump
	engine := db.GetEngine(db.DefaultContext).(*xorm.Engine)
	require.NoError(t, engine.DumpTables([]*schemas.Table{authSourceSchema}, sb))
	assert.Contains(t, sb.String(), `"Provider":"ConvertibleSourceName"`)
}
