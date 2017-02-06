// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/modules/markdown"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {
	Convey("The metas map", t, func() {
		var repo = new(Repository)
		repo.Name = "testrepo"
		repo.Owner = new(User)
		repo.Owner.Name = "testuser"
		externalTracker := RepoUnit{
			Type: UnitTypeExternalTracker,
			Config: &ExternalTrackerConfig{
				ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
			},
		}
		repo.Units = []*RepoUnit{
			&externalTracker,
		}

		Convey("When no external tracker is configured", func() {
			Convey("It should be nil", func() {
				repo.Units = nil
				So(repo.ComposeMetas(), ShouldEqual, map[string]string(nil))
			})
			Convey("It should be nil even if other settings are present", func() {
				repo.Units = nil
				So(repo.ComposeMetas(), ShouldEqual, map[string]string(nil))
			})
		})

		Convey("When an external issue tracker is configured", func() {
			repo.Units = []*RepoUnit{
				&externalTracker,
			}
			Convey("It should default to numeric issue style", func() {
				metas := repo.ComposeMetas()
				So(metas["style"], ShouldEqual, markdown.IssueNameStyleNumeric)
			})
			Convey("It should pass through numeric issue style setting", func() {
				externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markdown.IssueNameStyleNumeric
				metas := repo.ComposeMetas()
				So(metas["style"], ShouldEqual, markdown.IssueNameStyleNumeric)
			})
			Convey("It should pass through alphanumeric issue style setting", func() {
				externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markdown.IssueNameStyleAlphanumeric
				metas := repo.ComposeMetas()
				So(metas["style"], ShouldEqual, markdown.IssueNameStyleAlphanumeric)
			})
			Convey("It should contain the user name", func() {
				metas := repo.ComposeMetas()
				So(metas["user"], ShouldEqual, "testuser")
			})
			Convey("It should contain the repo name", func() {
				metas := repo.ComposeMetas()
				So(metas["repo"], ShouldEqual, "testrepo")
			})
			Convey("It should contain the URL format", func() {
				metas := repo.ComposeMetas()
				So(metas["format"], ShouldEqual, "https://someurl.com/{user}/{repo}/{issue}")
			})
		})
	})
}

func TestGetRepositoryCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err1 := GetRepositoryCount(&User{ID: int64(10)})
	privateCount, err2 := GetPrivateRepositoryCount(&User{ID: int64(10)})
	publicCount, err3 := GetPublicRepositoryCount(&User{ID: int64(10)})
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, (privateCount + publicCount), count)
}

func TestGetPublicRepositoryCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err := GetPublicRepositoryCount(&User{ID: int64(10)})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetPrivateRepositoryCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err := GetPrivateRepositoryCount(&User{ID: int64(10)})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
