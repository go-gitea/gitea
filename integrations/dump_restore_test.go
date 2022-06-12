// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/migrations"

	"github.com/stretchr/testify/assert"
	"lab.forgefriends.org/friendlyforgeformat/gofff"
	"lab.forgefriends.org/friendlyforgeformat/gofff/forges/file"
	"lab.forgefriends.org/friendlyforgeformat/gofff/format"
)

func TestDumpRestore(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		AllowLocalNetworks := setting.Migrations.AllowLocalNetworks
		setting.Migrations.AllowLocalNetworks = true
		AppVer := setting.AppVer
		// Gitea SDK (go-sdk) need to parse the AppVer from server response, so we must set it to a valid version string.
		setting.AppVer = "1.16.0"
		defer func() {
			setting.Migrations.AllowLocalNetworks = AllowLocalNetworks
			setting.AppVer = AppVer
		}()

		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
		session := loginUser(t, repoOwner.Name)
		token := getTokenForLoggedInUser(t, session)

		fixture := file.NewFixture(t, gofff.AllFeatures)
		fixture.CreateEverything(file.User{
			ID:    repoOwner.ID,
			Name:  repoOwner.Name,
			Email: repoOwner.Email,
		})

		assert.NoError(t, migrations.Init())

		ctx := context.Background()
		//
		// Phase 1: restore from the filesystem to the Gitea instance in restoredrepo
		//

		restoredRepoName := "restored"
		restoredRepoDirectory := fixture.GetDirectory()
		err := migrations.RestoreRepository(ctx, restoredRepoDirectory, repoOwner.Name, restoredRepoName, []string{
			"issues", "milestones", "labels", "releases", "release_assets", "comments", "pull_requests",
			// wiki",
		}, false)
		assert.NoError(t, err)

		restoredRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: restoredRepoName}).(*repo_model.Repository)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{Name: file.Asset1})

		//
		// Phase 2: dump restoredRepo from the Gitea instance to the filesystem
		//

		opts := base.MigrateOptions{
			GitServiceType: structs.GiteaService,

			Wiki:          true,
			Issues:        true,
			Milestones:    true,
			Labels:        true,
			Releases:      true,
			Comments:      true,
			PullRequests:  true,
			ReleaseAssets: true,

			AuthToken: token,
			CloneAddr: restoredRepo.CloneLink().HTTPS,
			RepoName:  restoredRepoName,
		}
		dumpedRepoDirectory := t.TempDir()
		err = migrations.DumpRepository(ctx, dumpedRepoDirectory, repoOwner.Name, opts)
		assert.NoError(t, err)

		//
		// Verify the dump of restored is the same as the dump of repo1
		//
		//fixture.AssertEquals(restoredRepoDirectory, dumpedRepoDirectory)
		//
		// Verify the fixture files are the same as the restored files
		//
		project := fixture.GetFile().GetProject()
		comparator := &compareDump{
			t: t,

			repoBefore:  project.Name,
			ownerBefore: project.Owner,
			dirBefore:   restoredRepoDirectory,

			repoAfter:  restoredRepoName,
			ownerAfter: repoOwner.Name,
			dirAfter:   dumpedRepoDirectory,
		}
		comparator.assertEquals()
	})
}

type compareDump struct {
	t *testing.T

	repoBefore  string
	ownerBefore string
	dirBefore   string

	repoAfter  string
	ownerAfter string
	dirAfter   string
}

type compareField struct {
	before    interface{}
	after     interface{}
	ignore    bool
	transform func(string) string
	nested    *compareFields
}

type compareFields map[string]compareField

func (c *compareDump) assertEquals() {
	//
	// base.Repository
	//
	_ = c.assertEqual("project.json", format.Project{}, compareFields{
		"Name": {
			before: c.repoBefore,
			after:  c.repoAfter,
		},
		"Owner": {
			before: c.ownerBefore,
			after:  c.ownerAfter,
		},
		"Index":    {ignore: true},
		"CloneURL": {ignore: true},
	})

	//
	// base.Label
	//
	compareLabels := compareFields{
		"Index": {ignore: true},
	}
	labels, ok := c.assertEqual("label.json", []format.Label{}, compareLabels).([]*format.Label)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(labels), 1)

	//
	// base.Milestone
	//
	milestones, ok := c.assertEqual("milestone.json", []format.Milestone{}, compareFields{
		"Index":   {ignore: true},
		"Updated": {ignore: true}, // the database updates that field independently
	}).([]*format.Milestone)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(milestones), 1)

	//
	// format.Issue and the associated comments
	//
	issues, ok := c.assertEqual("issue.json", []format.Issue{}, compareFields{
		"Index":     {ignore: true},
		"Assignees": {ignore: true}, // not implemented yet
		"Labels":    {nested: &compareLabels},
	}).([]*format.Issue)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(issues), 1)
	for _, issue := range issues {
		filename := filepath.Join("comments", fmt.Sprintf("%d.json", issue.Number))
		comments, ok := c.assertEqual(filename, []format.Comment{}, compareFields{
			"Index": {ignore: true},
		}).([]*format.Comment)
		assert.True(c.t, ok)
		for _, comment := range comments {
			assert.EqualValues(c.t, issue.Number, comment.IssueIndex)
		}
	}

	//
	// format.PullRequest and the associated comments
	//
	comparePullRequestBranch := &compareFields{
		"RepoName": {
			before: c.repoBefore,
			after:  c.repoAfter,
		},
		"OwnerName": {
			before: c.ownerBefore,
			after:  c.ownerAfter,
		},
		"CloneURL": {ignore: true},
	}
	prs, ok := c.assertEqual("pull_request.json", []format.PullRequest{}, compareFields{
		"Assignees": {ignore: true}, // not implemented yet
		"Head":      {nested: comparePullRequestBranch},
		"Base":      {nested: comparePullRequestBranch},
		"PatchURL":  {ignore: true},
		"CloneURL":  {ignore: true},
		"Labels":    {ignore: true}, // because org labels are not handled properly
	}).([]*format.PullRequest)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(prs), 1)
	for _, pr := range prs {
		filename := filepath.Join("comments", fmt.Sprintf("%d.json", pr.Number))
		comments, ok := c.assertEqual(filename, []format.Comment{}, compareFields{}).([]*format.Comment)
		assert.True(c.t, ok)
		for _, comment := range comments {
			assert.EqualValues(c.t, pr.Number, comment.IssueIndex)
		}
	}
}

func (c *compareDump) assertLoadJSONFiles(beforeFilename, afterFilename string, before, after interface{}) {
	_, beforeErr := os.Stat(beforeFilename)
	_, afterErr := os.Stat(afterFilename)
	assert.EqualValues(c.t, errors.Is(beforeErr, os.ErrNotExist), errors.Is(afterErr, os.ErrNotExist))
	if errors.Is(beforeErr, os.ErrNotExist) {
		return
	}

	beforeBytes, err := os.ReadFile(beforeFilename)
	assert.NoError(c.t, err)
	assert.NoError(c.t, json.Unmarshal(beforeBytes, before))
	afterBytes, err := os.ReadFile(afterFilename)
	assert.NoError(c.t, err)
	assert.NoError(c.t, json.Unmarshal(afterBytes, after))
}

func (c *compareDump) assertLoadFiles(beforeFilename, afterFilename string, t reflect.Type) (before, after reflect.Value) {
	var beforePtr, afterPtr reflect.Value
	if t.Kind() == reflect.Slice {
		//
		// Given []Something{} create afterPtr, beforePtr []*Something{}
		//
		sliceType := reflect.SliceOf(reflect.PtrTo(t.Elem()))
		beforeSlice := reflect.MakeSlice(sliceType, 0, 10)
		beforePtr = reflect.New(beforeSlice.Type())
		beforePtr.Elem().Set(beforeSlice)
		afterSlice := reflect.MakeSlice(sliceType, 0, 10)
		afterPtr = reflect.New(afterSlice.Type())
		afterPtr.Elem().Set(afterSlice)
	} else {
		//
		// Given Something{} create afterPtr, beforePtr *Something{}
		//
		beforePtr = reflect.New(t)
		afterPtr = reflect.New(t)
	}
	c.assertLoadJSONFiles(beforeFilename, afterFilename, beforePtr.Interface(), afterPtr.Interface())
	return beforePtr.Elem(), afterPtr.Elem()
}

func (c *compareDump) assertEqual(filename string, kind interface{}, fields compareFields) (i interface{}) {
	beforeFilename := filepath.Join(c.dirBefore, filename)
	afterFilename := filepath.Join(c.dirAfter, filename)
	fmt.Println("assertEqual ", beforeFilename, afterFilename)

	typeOf := reflect.TypeOf(kind)
	before, after := c.assertLoadFiles(beforeFilename, afterFilename, typeOf)
	if typeOf.Kind() == reflect.Slice {
		i = c.assertEqualSlices(before, after, fields)
	} else {
		i = c.assertEqualValues(before, after, fields)
	}
	return i
}

func (c *compareDump) assertEqualSlices(before, after reflect.Value, fields compareFields) interface{} {
	assert.EqualValues(c.t, before.Len(), after.Len())
	if before.Len() == after.Len() {
		for i := 0; i < before.Len(); i++ {
			_ = c.assertEqualValues(
				reflect.Indirect(before.Index(i).Elem()),
				reflect.Indirect(after.Index(i).Elem()),
				fields)
		}
	}
	return after.Interface()
}

func (c *compareDump) assertEqualValues(before, after reflect.Value, fields compareFields) interface{} {
	for _, field := range reflect.VisibleFields(before.Type()) {
		bf := before.FieldByName(field.Name)
		bi := bf.Interface()
		af := after.FieldByName(field.Name)
		ai := af.Interface()
		if compare, ok := fields[field.Name]; ok {
			if compare.ignore == true {
				//
				// Ignore
				//
				continue
			}
			if compare.transform != nil {
				//
				// Transform these strings before comparing them
				//
				bs, ok := bi.(string)
				assert.True(c.t, ok, field.Name)
				as, ok := ai.(string)
				assert.True(c.t, ok, field.Name)
				assert.EqualValues(c.t, compare.transform(bs), compare.transform(as), field.Name)
				continue
			}
			if compare.before != nil && compare.after != nil {
				//
				// The fields are expected to have different values
				//
				assert.EqualValues(c.t, compare.before, bi, field.Name)
				assert.EqualValues(c.t, compare.after, ai, field.Name)
				continue
			}
			if compare.nested != nil {
				//
				// The fields are a struct/slice, recurse
				//
				fmt.Println("nested ", field.Name)
				if reflect.TypeOf(bi).Kind() == reflect.Slice {
					c.assertEqualSlices(bf, af, *compare.nested)
				} else {
					c.assertEqualValues(bf, af, *compare.nested)
				}
				continue
			}
		}
		assert.EqualValues(c.t, bi, ai, field.Name)
	}
	return after.Interface()
}
