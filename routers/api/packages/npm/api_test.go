// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"testing"
	"time"

	packages_model "gitea.dev/models/packages"
	user_model "gitea.dev/models/user"
	npm_module "gitea.dev/modules/packages/npm"
	"gitea.dev/modules/timeutil"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func TestCreatePackageMetadataResponse(t *testing.T) {
	descriptor := func(v string, publishedUnix int64) *packages_model.PackageDescriptor {
		return &packages_model.PackageDescriptor{
			Package:  &packages_model.Package{Name: "test"},
			Owner:    &user_model.User{Name: "alice"},
			Version:  &packages_model.PackageVersion{Version: v, CreatedUnix: timeutil.TimeStamp(publishedUnix)},
			SemVer:   version.Must(version.NewVersion(v)),
			Metadata: &npm_module.Metadata{Keywords: []string{"gitea"}},
			Files:    []*packages_model.PackageFileDescriptor{{File: &packages_model.PackageFile{}, Blob: &packages_model.PackageBlob{}}},
		}
	}

	result := createPackageMetadataResponse("https://gitea.dev/api/packages/alice/npm", []*packages_model.PackageDescriptor{
		descriptor("1.1.0", 1000),
		descriptor("1.0.0", 2000),
	})

	assert.Equal(t, map[string]time.Time{
		"1.0.0":    time.Unix(2000, 0).UTC(),
		"1.1.0":    time.Unix(1000, 0).UTC(),
		"created":  time.Unix(1000, 0).UTC(),
		"modified": time.Unix(2000, 0).UTC(),
	}, result.Time)
	assert.Equal(t, []npm_module.User{{Name: "alice"}}, result.Maintainers)
	assert.Equal(t, []string{"gitea"}, result.Keywords)
	assert.Equal(t, []string{"gitea"}, result.Versions["1.0.0"].Keywords)
	assert.Equal(t, []npm_module.User{{Name: "alice"}}, result.Versions["1.0.0"].Maintainers)
}

func TestCreatePackageMetadataVersion(t *testing.T) {
	descriptor := func(name, ver, fileName string) *packages_model.PackageDescriptor {
		return &packages_model.PackageDescriptor{
			Package:  &packages_model.Package{Name: name},
			Owner:    &user_model.User{Name: "alice"},
			Version:  &packages_model.PackageVersion{Version: ver},
			SemVer:   version.Must(version.NewVersion(ver)),
			Metadata: &npm_module.Metadata{},
			Files: []*packages_model.PackageFileDescriptor{{
				File: &packages_model.PackageFile{LowerName: fileName},
				Blob: &packages_model.PackageBlob{},
			}},
		}
	}

	const registryURL = "https://gitea.dev/api/packages/alice/npm"

	t.Run("scoped", func(t *testing.T) {
		// url.QueryEscape would leave '@' unescaped and emit '@scope%2Fname',
		// which npm clients cannot resolve. url.PathEscape emits the
		// RFC 3986 path-segment-safe '@scope%2Fname' that npm registry URLs require.
		got := createPackageMetadataVersion(registryURL, descriptor("@scope/name", "1.0.0", "name-1.0.0.tgz"))
		assert.Equal(t, registryURL+"/@scope%2Fname/-/1.0.0/name-1.0.0.tgz", got.Dist.Tarball)
	})

	t.Run("unscoped", func(t *testing.T) {
		got := createPackageMetadataVersion(registryURL, descriptor("name", "1.0.0", "name-1.0.0.tgz"))
		assert.Equal(t, registryURL+"/name/-/1.0.0/name-1.0.0.tgz", got.Dist.Tarball)
	})
}
