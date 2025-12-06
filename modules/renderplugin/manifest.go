// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

var identifierRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9\-_.]{1,63}$`)

// Manifest describes the metadata declared by a render plugin.
const SupportedManifestVersion = 1

type Manifest struct {
	SchemaVersion int      `json:"schemaVersion"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Description   string   `json:"description"`
	Entry         string   `json:"entry"`
	FilePatterns  []string `json:"filePatterns"`
}

// Normalize validates mandatory fields and normalizes values.
func (m *Manifest) Normalize() error {
	if m.SchemaVersion == 0 {
		return fmt.Errorf("manifest schemaVersion is required")
	}
	if m.SchemaVersion != SupportedManifestVersion {
		return fmt.Errorf("manifest schemaVersion %d is not supported", m.SchemaVersion)
	}
	m.ID = strings.TrimSpace(strings.ToLower(m.ID))
	if !identifierRegexp.MatchString(m.ID) {
		return fmt.Errorf("manifest id %q is invalid; only lowercase letters, numbers, dash, underscore and dot are allowed", m.ID)
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}
	m.Version = strings.TrimSpace(m.Version)
	if m.Version == "" {
		return fmt.Errorf("manifest version is required")
	}
	if m.Entry == "" {
		m.Entry = "render.js"
	}
	m.Entry = util.PathJoinRelX(m.Entry)
	if m.Entry == "" || strings.HasPrefix(m.Entry, "../") {
		return fmt.Errorf("manifest entry %q is invalid", m.Entry)
	}
	cleanPatterns := make([]string, 0, len(m.FilePatterns))
	for _, pattern := range m.FilePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		cleanPatterns = append(cleanPatterns, pattern)
	}
	if len(cleanPatterns) == 0 {
		return fmt.Errorf("manifest must declare at least one file pattern")
	}
	sort.Strings(cleanPatterns)
	m.FilePatterns = cleanPatterns
	return nil
}

// LoadManifest reads and validates the manifest.json file located under dir.
func LoadManifest(dir string) (*Manifest, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	f, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var manifest Manifest
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("malformed manifest.json: %w", err)
	}
	if err := manifest.Normalize(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// Metadata is the public information exposed to the frontend for an enabled plugin.
type Metadata struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Entry        string   `json:"entry"`
	EntryURL     string   `json:"entryUrl"`
	AssetsBase   string   `json:"assetsBaseUrl"`
	FilePatterns []string `json:"filePatterns"`
	SchemaVersion int     `json:"schemaVersion"`
}
