// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/migrations"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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

		assert.NoError(t, migrations.Init())

		reponame := "repo1"

		basePath := t.TempDir()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame})
		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, repoOwner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeReadMisc)

		//
		// Phase 1: dump repo1 from the Gitea instance to the filesystem
		//

		ctx := t.Context()
		opts := migrations.MigrateOptions{
			GitServiceType: structs.GiteaService,
			Issues:         true,
			PullRequests:   true,
			Labels:         true,
			Milestones:     true,
			Comments:       true,
			AuthToken:      token,
			CloneAddr:      repo.CloneLinkGeneral(t.Context()).HTTPS,
			RepoName:       reponame,
		}
		err := migrations.DumpRepository(ctx, basePath, repoOwner.Name, opts)
		assert.NoError(t, err)

		//
		// Verify desired side effects of the dump
		//
		d := filepath.Join(basePath, repo.OwnerName, repo.Name)
		for _, f := range []string{"repo.yml", "topic.yml", "label.yml", "milestone.yml", "issue.yml"} {
			assert.FileExists(t, filepath.Join(d, f))
		}

		//
		// Phase 2: restore from the filesystem to the Gitea instance in restoredrepo
		//

		newreponame := "restored"
		err = migrations.RestoreRepository(ctx, d, repo.OwnerName, newreponame, []string{
			"labels", "issues", "comments", "milestones", "pull_requests",
		}, false)
		assert.NoError(t, err)

		newrepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: newreponame})

		//
		// Phase 3: dump restored from the Gitea instance to the filesystem
		//
		opts.RepoName = newreponame
		opts.CloneAddr = newrepo.CloneLinkGeneral(t.Context()).HTTPS
		err = migrations.DumpRepository(ctx, basePath, repoOwner.Name, opts)
		assert.NoError(t, err)

		//
		// Verify the dump of restored is the same as the dump of repo1
		//
		comparator := &compareDump{
			t:        t,
			basePath: basePath,
		}
		comparator.assertEquals(repo, newrepo)
	})
}

type compareDump struct {
	t          *testing.T
	basePath   string
	repoBefore *repo_model.Repository
	dirBefore  string
	repoAfter  *repo_model.Repository
	dirAfter   string
}

type compareField struct {
	before    any
	after     any
	ignore    bool
	transform func(string) string
	nested    *compareFields
}

type compareFields map[string]compareField

func (c *compareDump) replaceRepoName(original string) string {
	return strings.ReplaceAll(original, c.repoBefore.Name, c.repoAfter.Name)
}

func (c *compareDump) assertEquals(repoBefore, repoAfter *repo_model.Repository) {
	c.repoBefore = repoBefore
	c.dirBefore = filepath.Join(c.basePath, repoBefore.OwnerName, repoBefore.Name)
	c.repoAfter = repoAfter
	c.dirAfter = filepath.Join(c.basePath, repoAfter.OwnerName, repoAfter.Name)

	//
	// base.Repository
	//
	_ = c.assertEqual("repo.yml", base.Repository{}, compareFields{
		"Name": {
			before: c.repoBefore.Name,
			after:  c.repoAfter.Name,
		},
		"CloneURL":    {transform: c.replaceRepoName},
		"OriginalURL": {transform: c.replaceRepoName},
	})

	//
	// base.Label
	//
	labels, ok := c.assertEqual("label.yml", []base.Label{}, compareFields{}).([]*base.Label)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(labels), 1)

	//
	// base.Milestone
	//
	milestones, ok := c.assertEqual("milestone.yml", []base.Milestone{}, compareFields{
		"Updated": {ignore: true}, // the database updates that field independently
	}).([]*base.Milestone)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(milestones), 1)

	//
	// base.Issue and the associated comments
	//
	issues, ok := c.assertEqual("issue.yml", []base.Issue{}, compareFields{
		"Assignees": {ignore: true}, // not implemented yet
	}).([]*base.Issue)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(issues), 1)
	for _, issue := range issues {
		filename := filepath.Join("comments", fmt.Sprintf("%d.yml", issue.Number))
		comments, ok := c.assertEqual(filename, []base.Comment{}, compareFields{
			"Index": {ignore: true},
		}).([]*base.Comment)
		assert.True(c.t, ok)
		for _, comment := range comments {
			assert.EqualValues(c.t, issue.Number, comment.IssueIndex)
		}
	}

	//
	// base.PullRequest and the associated comments
	//
	comparePullRequestBranch := &compareFields{
		"RepoName": {
			before: c.repoBefore.Name,
			after:  c.repoAfter.Name,
		},
		"CloneURL": {transform: c.replaceRepoName},
	}
	prs, ok := c.assertEqual("pull_request.yml", []base.PullRequest{}, compareFields{
		"Assignees": {ignore: true}, // not implemented yet
		"Head":      {nested: comparePullRequestBranch},
		"Base":      {nested: comparePullRequestBranch},
		"Labels":    {ignore: true}, // because org labels are not handled properly
	}).([]*base.PullRequest)
	assert.True(c.t, ok)
	assert.GreaterOrEqual(c.t, len(prs), 1)
	for _, pr := range prs {
		filename := filepath.Join("comments", fmt.Sprintf("%d.yml", pr.Number))
		comments, ok := c.assertEqual(filename, []base.Comment{}, compareFields{}).([]*base.Comment)
		assert.True(c.t, ok)
		for _, comment := range comments {
			assert.EqualValues(c.t, pr.Number, comment.IssueIndex)
		}
	}
}

func (c *compareDump) assertLoadYAMLFiles(beforeFilename, afterFilename string, before, after any) {
	_, beforeErr := os.Stat(beforeFilename)
	_, afterErr := os.Stat(afterFilename)
	assert.EqualValues(c.t, errors.Is(beforeErr, os.ErrNotExist), errors.Is(afterErr, os.ErrNotExist))
	if errors.Is(beforeErr, os.ErrNotExist) {
		return
	}

	beforeBytes, err := os.ReadFile(beforeFilename)
	assert.NoError(c.t, err)
	assert.NoError(c.t, yaml.Unmarshal(beforeBytes, before))
	afterBytes, err := os.ReadFile(afterFilename)
	assert.NoError(c.t, err)
	assert.NoError(c.t, yaml.Unmarshal(afterBytes, after))
}

func (c *compareDump) assertLoadFiles(beforeFilename, afterFilename string, t reflect.Type) (before, after reflect.Value) {
	var beforePtr, afterPtr reflect.Value
	if t.Kind() == reflect.Slice {
		//
		// Given []Something{} create afterPtr, beforePtr []*Something{}
		//
		sliceType := reflect.SliceOf(reflect.PointerTo(t.Elem()))
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
	c.assertLoadYAMLFiles(beforeFilename, afterFilename, beforePtr.Interface(), afterPtr.Interface())
	return beforePtr.Elem(), afterPtr.Elem()
}

func (c *compareDump) assertEqual(filename string, kind any, fields compareFields) (i any) {
	beforeFilename := filepath.Join(c.dirBefore, filename)
	afterFilename := filepath.Join(c.dirAfter, filename)

	typeOf := reflect.TypeOf(kind)
	before, after := c.assertLoadFiles(beforeFilename, afterFilename, typeOf)
	if typeOf.Kind() == reflect.Slice {
		i = c.assertEqualSlices(before, after, fields)
	} else {
		i = c.assertEqualValues(before, after, fields)
	}
	return i
}

func (c *compareDump) assertEqualSlices(before, after reflect.Value, fields compareFields) any {
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

func (c *compareDump) assertEqualValues(before, after reflect.Value, fields compareFields) any {
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
				assert.True(c.t, ok)
				as, ok := ai.(string)
				assert.True(c.t, ok)
				assert.EqualValues(c.t, compare.transform(bs), compare.transform(as))
				continue
			}
			if compare.before != nil && compare.after != nil {
				//
				// The fields are expected to have different values
				//
				assert.EqualValues(c.t, compare.before, bi)
				assert.EqualValues(c.t, compare.after, ai)
				continue
			}
			if compare.nested != nil {
				//
				// The fields are a struct, recurse
				//
				c.assertEqualValues(bf, af, *compare.nested)
				continue
			}
		}
		assert.EqualValues(c.t, bi, ai)
	}
	return after.Interface()
}
