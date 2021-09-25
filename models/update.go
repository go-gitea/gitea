// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
)

// PushUpdateDeleteTagsContext updates a number of delete tags with context
func PushUpdateDeleteTagsContext(ctx context.Context, repo *Repository, tags []string) error {
	return pushUpdateDeleteTags(db.GetEngine(ctx), repo, tags)
}

func pushUpdateDeleteTags(e db.Engine, repo *Repository, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	if _, err := e.
		Where("repo_id = ? AND is_tag = ?", repo.ID, true).
		In("lower_tag_name", lowerTags).
		Delete(new(Release)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	if _, err := e.
		Where("repo_id = ? AND is_tag = ?", repo.ID, false).
		In("lower_tag_name", lowerTags).
		Cols("is_draft", "num_commits", "sha1").
		Update(&Release{
			IsDraft: true,
		}); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

// PushUpdateDeleteTag must be called for any push actions to delete tag
func PushUpdateDeleteTag(repo *Repository, tagName string) error {
	rel, err := GetRelease(repo.ID, tagName)
	if err != nil {
		if IsErrReleaseNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetRelease: %v", err)
	}
	if rel.IsTag {
		if _, err = db.GetEngine(db.DefaultContext).ID(rel.ID).Delete(new(Release)); err != nil {
			return fmt.Errorf("Delete: %v", err)
		}
	} else {
		rel.IsDraft = true
		rel.NumCommits = 0
		rel.Sha1 = ""
		if _, err = db.GetEngine(db.DefaultContext).ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}

	return nil
}

// SaveOrUpdateTag must be called for any push actions to add tag
func SaveOrUpdateTag(repo *Repository, newRel *Release) error {
	rel, err := GetRelease(repo.ID, newRel.TagName)
	if err != nil && !IsErrReleaseNotExist(err) {
		return fmt.Errorf("GetRelease: %v", err)
	}

	if rel == nil {
		rel = newRel
		if _, err = db.GetEngine(db.DefaultContext).Insert(rel); err != nil {
			return fmt.Errorf("InsertOne: %v", err)
		}
	} else {
		rel.Sha1 = newRel.Sha1
		rel.CreatedUnix = newRel.CreatedUnix
		rel.NumCommits = newRel.NumCommits
		rel.IsDraft = false
		if rel.IsTag && newRel.PublisherID > 0 {
			rel.PublisherID = newRel.PublisherID
		}
		if _, err = db.GetEngine(db.DefaultContext).ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}
	return nil
}
