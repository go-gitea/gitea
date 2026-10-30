// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLegacyConfigJSON(t *testing.T) {
	const (
		config = `{"Host":"smtp.example.com"}`
		packed = "≻潈瑳㨢猢瑭⹰硥浡汰⹥潣≭⁽"
		valid  = ` {"name":"例😀","id":9007199254740993} `
	)
	for input, want := range map[string]string{
		"":                          "",
		valid:                       valid,
		packed:                      config,
		"\ufeff" + packed:           config,
		"\xff\xfe" + config:         config,
		"\xff\xfe" + `{"name":"😀"}`: `{"name":"😀"}`,
		"invalid":                   "invalid",
		"invalid😀":                  "invalid😀",
	} {
		assert.Equal(t, want, normalizeLegacyConfigJSON(input), "input: %q", input)
	}

	type RepoUnit struct {
		ID     int64  `xorm:"pk autoincr"`
		Config string `xorm:"TEXT"`
	}
	type LoginSource struct {
		ID  int64  `xorm:"pk autoincr"`
		Cfg string `xorm:"TEXT"`
	}
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(RepoUnit), new(LoginSource))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	defer test.MockVariableValue(&setting.Database.IterateBufferSize, 2)()

	for _, input := range []string{valid, packed} {
		_, err := x.Insert(&RepoUnit{Config: input})
		require.NoError(t, err)
		_, err = x.Insert(&LoginSource{Cfg: input})
		require.NoError(t, err)
	}
	_, err := x.Exec("INSERT INTO `repo_unit` (`config`) VALUES (NULL)")
	require.NoError(t, err)
	_, err = x.Exec("INSERT INTO `login_source` (`cfg`) VALUES (NULL)")
	require.NoError(t, err)

	require.NoError(t, NormalizeLegacyConfigJSON(t.Context(), x))
	var units []RepoUnit
	require.NoError(t, x.Asc("id").Find(&units))
	var sources []LoginSource
	require.NoError(t, x.Asc("id").Find(&sources))
	want := []string{valid, config, ""}
	require.Len(t, units, len(want))
	require.Len(t, sources, len(want))
	for i, expected := range want {
		assert.Equal(t, expected, units[i].Config)
		assert.Equal(t, expected, sources[i].Cfg)
	}
}
