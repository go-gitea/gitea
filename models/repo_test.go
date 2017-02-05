package models_test

import (
	"testing"

	. "code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/markdown"
	. "github.com/smartystreets/goconvey/convey"
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
