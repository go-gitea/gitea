package renderplugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestNormalizeDefaults(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SupportedManifestVersion,
		ID:            " Example.Plugin ",
		Name:          " Demo Plugin ",
		Version:       " 1.0.0 ",
		Description:   "test",
		Entry:         "",
		FilePatterns:  []string{"  *.TXT  ", "README.md", ""},
	}

	require.NoError(t, manifest.Normalize())
	assert.Equal(t, "example.plugin", manifest.ID)
	assert.Equal(t, "render.js", manifest.Entry)
	assert.Equal(t, []string{"*.TXT", "README.md"}, manifest.FilePatterns)
}

func TestManifestNormalizeErrors(t *testing.T) {
	base := Manifest{
		SchemaVersion: SupportedManifestVersion,
		ID:            "example",
		Name:          "demo",
		Version:       "1.0",
		Entry:         "render.js",
		FilePatterns:  []string{"*.md"},
	}

	tests := []struct {
		name    string
		mutate  func(m *Manifest)
		message string
	}{
		{"missing schema version", func(m *Manifest) { m.SchemaVersion = 0 }, "schemaVersion is required"},
		{"unsupported schema", func(m *Manifest) { m.SchemaVersion = SupportedManifestVersion + 1 }, "not supported"},
		{"invalid id", func(m *Manifest) { m.ID = "bad id" }, "manifest id"},
		{"missing name", func(m *Manifest) { m.Name = "" }, "name is required"},
		{"missing version", func(m *Manifest) { m.Version = "" }, "version is required"},
		{"no patterns", func(m *Manifest) { m.FilePatterns = nil }, "at least one file pattern"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := base
			tt.mutate(&m)
			err := m.Normalize()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	manifestJSON := `{
		"schemaVersion": 1,
		"id": "Example",
		"name": "Example",
		"version": "2.0.0",
		"description": "demo",
		"entry": "render.js",
		"filePatterns": ["*.txt", "*.md"]
	}`
	path := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(manifestJSON), 0o644))

	manifest, err := LoadManifest(dir)
	require.NoError(t, err)
	assert.Equal(t, "example", manifest.ID)
	assert.Equal(t, []string{"*.md", "*.txt"}, manifest.FilePatterns)
}
