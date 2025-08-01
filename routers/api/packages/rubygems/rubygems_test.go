// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rubygems

import (
	"strings"
	"testing"

	rubygems_module "code.gitea.io/gitea/modules/packages/rubygems"

	"github.com/stretchr/testify/assert"
)

func TestWritePackageVersion(t *testing.T) {
	buf := &strings.Builder{}

	writePackageVersionForList(nil, "1.0", " ", buf)
	assert.Equal(t, "1.0 ", buf.String())
	buf.Reset()

	writePackageVersionForList(&rubygems_module.Metadata{Platform: "ruby"}, "1.0", " ", buf)
	assert.Equal(t, "1.0 ", buf.String())
	buf.Reset()

	writePackageVersionForList(&rubygems_module.Metadata{Platform: "linux"}, "1.0", " ", buf)
	assert.Equal(t, "1.0_linux ", buf.String())
	buf.Reset()

	writePackageVersionForDependency("1.0", "", buf)
	assert.Equal(t, "1.0 ", buf.String())
	buf.Reset()

	writePackageVersionForDependency("1.0", "ruby", buf)
	assert.Equal(t, "1.0 ", buf.String())
	buf.Reset()

	writePackageVersionForDependency("1.0", "os", buf)
	assert.Equal(t, "1.0-os ", buf.String())
	buf.Reset()
}
