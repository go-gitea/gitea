// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rubygems

import (
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestParsePackageMetaData(t *testing.T) {
	t.Run("MissingMetadataFile", func(t *testing.T) {
		data := test.WriteTarArchive(map[string]string{"dummy.txt": ""})
		rp, err := ParsePackageMetaData(data)
		assert.ErrorIs(t, err, ErrMissingMetadataFile)
		assert.Nil(t, rp)
	})

	t.Run("Valid", func(t *testing.T) {
		metadataContent := test.CompressGzip(`
name: g
version:
  version: 1
`)
		data := test.WriteTarArchive(map[string]string{
			"metadata.gz": metadataContent.String(),
		})
		rp, err := ParsePackageMetaData(data)
		assert.NoError(t, err)
		assert.NotNil(t, rp)
	})
}

func TestParseMetadataFile(t *testing.T) {
	content := test.CompressGzip(`--- !ruby/object:Gem::Specification
name: gitea
version: !ruby/object:Gem::Version
  version: 1.0.5
platform: ruby
authors:
- Gitea
autorequire:
bindir: bin
cert_chain: []
date: 2021-08-23 00:00:00.000000000 Z
dependencies:
- !ruby/object:Gem::Dependency
  name: runtime-dep
  requirement: !ruby/object:Gem::Requirement
    requirements:
    - - ">="
      - !ruby/object:Gem::Version
        version: 1.2.0
    - - "<"
      - !ruby/object:Gem::Version
        version: '2.0'
  type: :runtime
  prerelease: false
  version_requirements: !ruby/object:Gem::Requirement
    requirements:
    - - ">="
      - !ruby/object:Gem::Version
        version: 1.2.0
    - - "<"
      - !ruby/object:Gem::Version
        version: '2.0'
- !ruby/object:Gem::Dependency
  name: dev-dep
  requirement: !ruby/object:Gem::Requirement
    requirements:
    - - "~>"
      - !ruby/object:Gem::Version
        version: '0'
  type: :development
  prerelease: false
  version_requirements: !ruby/object:Gem::Requirement
    requirements:
    - - "~>"
      - !ruby/object:Gem::Version
        version: '5.2'
description: RubyGems package test
email: rubygems@gitea.io
executables: []
extensions: []
extra_rdoc_files: []
files:
- lib/gitea.rb
homepage: https://gitea.io/
licenses:
- MIT
metadata: {}
post_install_message:
rdoc_options: []
require_paths:
- lib
required_ruby_version: !ruby/object:Gem::Requirement
  requirements:
  - - ">="
    - !ruby/object:Gem::Version
      version: 2.3.0
required_rubygems_version: !ruby/object:Gem::Requirement
  requirements:
  - - ">="
    - !ruby/object:Gem::Version
      version: '0'
requirements: []
rubyforge_project:
rubygems_version: 2.7.6.2
signing_key:
specification_version: 4
summary: Gitea package
test_files: []
`)
	rp, err := parseMetadataFile(content)
	assert.NoError(t, err)
	assert.NotNil(t, rp)

	assert.Equal(t, "gitea", rp.Name)
	assert.Equal(t, "1.0.5", rp.Version)
	assert.Equal(t, "ruby", rp.Metadata.Platform)
	assert.Equal(t, "Gitea package", rp.Metadata.Summary)
	assert.Equal(t, "RubyGems package test", rp.Metadata.Description)
	assert.Equal(t, []string{"Gitea"}, rp.Metadata.Authors)
	assert.Equal(t, "https://gitea.io/", rp.Metadata.ProjectURL)
	assert.Equal(t, []string{"MIT"}, rp.Metadata.Licenses)
	assert.Empty(t, rp.Metadata.RequiredRubygemsVersion)
	assert.Len(t, rp.Metadata.RequiredRubyVersion, 1)
	assert.Equal(t, ">=", rp.Metadata.RequiredRubyVersion[0].Restriction)
	assert.Equal(t, "2.3.0", rp.Metadata.RequiredRubyVersion[0].Version)
	assert.Len(t, rp.Metadata.RuntimeDependencies, 1)
	assert.Equal(t, "runtime-dep", rp.Metadata.RuntimeDependencies[0].Name)
	assert.Len(t, rp.Metadata.RuntimeDependencies[0].Version, 2)
	assert.Equal(t, ">=", rp.Metadata.RuntimeDependencies[0].Version[0].Restriction)
	assert.Equal(t, "1.2.0", rp.Metadata.RuntimeDependencies[0].Version[0].Version)
	assert.Equal(t, "<", rp.Metadata.RuntimeDependencies[0].Version[1].Restriction)
	assert.Equal(t, "2.0", rp.Metadata.RuntimeDependencies[0].Version[1].Version)
	assert.Len(t, rp.Metadata.DevelopmentDependencies, 1)
	assert.Equal(t, "dev-dep", rp.Metadata.DevelopmentDependencies[0].Name)
	assert.Len(t, rp.Metadata.DevelopmentDependencies[0].Version, 1)
	assert.Equal(t, "~>", rp.Metadata.DevelopmentDependencies[0].Version[0].Restriction)
	assert.Equal(t, "0", rp.Metadata.DevelopmentDependencies[0].Version[0].Version)
}
