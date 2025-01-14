// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseMetadataFromState tests the ParseMetadataFromState function
func TestParseMetadataFromState(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectedError bool
	}{
		{
			name:          "valid state file",
			input:         createValidStateArchive(),
			expectedError: false,
		},
		{
			name:          "missing state.json file",
			input:         createInvalidStateArchive(),
			expectedError: true,
		},
		{
			name:          "corrupt archive",
			input:         []byte("invalid archive data"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			metadata, err := ParseMetadataFromState(r)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, metadata)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metadata)
				// Optionally, check if certain fields are populated correctly
				assert.NotEmpty(t, metadata.Lineage)
			}
		})
	}
}

// createValidStateArchive creates a valid TAR.GZ archive with a sample state.json
func createValidStateArchive() []byte {
	metadata := `{
		"version": 4,
		"terraform_version": "1.2.0",
		"serial": 1,
		"lineage": "abc123",
		"resources": [],
		"description": "Test project",
		"author": "Test Author",
		"project_url": "http://example.com",
		"repository_url": "http://repo.com"
	}`

	// Create a gzip writer and tar writer
	buf := new(bytes.Buffer)
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)

	// Add the state.json file to the tar
	hdr := &tar.Header{
		Name: "state.json",
		Size: int64(len(metadata)),
		Mode: 0o600,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		panic(err)
	}
	if _, err := tw.Write([]byte(metadata)); err != nil {
		panic(err)
	}

	// Close the writers
	if err := tw.Close(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// createInvalidStateArchive creates an invalid TAR.GZ archive (missing state.json)
func createInvalidStateArchive() []byte {
	// Create a tar archive without the state.json file
	buf := new(bytes.Buffer)
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)

	// Add an empty file to the tar (but not state.json)
	hdr := &tar.Header{
		Name: "other_file.txt",
		Size: 0,
		Mode: 0o600,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		panic(err)
	}

	// Close the writers
	if err := tw.Close(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// TestParseStateFile tests the ParseStateFile function directly
func TestParseStateFile(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError bool
	}{
		{
			name:          "valid state.json",
			input:         `{"version":4,"terraform_version":"1.2.0","serial":1,"lineage":"abc123"}`,
			expectedError: false,
		},
		{
			name:          "invalid JSON",
			input:         `{"version":4,"terraform_version"}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tt.input))
			metadata, err := ParseStateFile(r)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, metadata)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metadata)
			}
		})
	}
}
