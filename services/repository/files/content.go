// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"net/url"
	"path"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ContentType repo content type
type ContentType string

// The string representations of different content types
const (
	ContentTypeRegular   ContentType = "file"      // regular content type (file)
	ContentTypeDir       ContentType = "dir"       // dir content type (dir)
	ContentTypeLink      ContentType = "symlink"   // link content type (symlink)
	ContentTypeSubmodule ContentType = "submodule" // submodule content type (submodule)
)

// String gets the string of ContentType
func (ct *ContentType) String() string {
	return string(*ct)
}

// GetContentsOrList gets the metadata of a file's contents (*ContentsResponse) if treePath not a tree
// directory, otherwise a listing of file contents ([]*ContentsResponse). Ref can be a branch, commit or tag
func GetContentsOrList(ctx context.Context, repo *repo_model.Repository, refCommit *utils.RefCommit, treePath string) (any, error) {
	if repo.IsEmpty {
		return make([]any, 0), nil
	}

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	// Get the commit object for the ref
	commit := refCommit.Commit

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	if entry.Type() != "tree" {
		return GetContents(ctx, repo, refCommit, treePath, false)
	}

	// We are in a directory, so we return a list of FileContentResponse objects
	var fileList []*api.ContentsResponse

	gitTree, err := commit.SubTree(treePath)
	if err != nil {
		return nil, err
	}
	entries, err := gitTree.ListEntries()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		subTreePath := path.Join(treePath, e.Name())
		fileContentResponse, err := GetContents(ctx, repo, refCommit, subTreePath, true)
		if err != nil {
			return nil, err
		}
		fileList = append(fileList, fileContentResponse)
	}
	return fileList, nil
}

// GetObjectTypeFromTreeEntry check what content is behind it
func GetObjectTypeFromTreeEntry(entry *git.TreeEntry) ContentType {
	switch {
	case entry.IsDir():
		return ContentTypeDir
	case entry.IsSubModule():
		return ContentTypeSubmodule
	case entry.IsExecutable(), entry.IsRegular():
		return ContentTypeRegular
	case entry.IsLink():
		return ContentTypeLink
	default:
		return ""
	}
}

// GetContents gets the metadata on a file's contents. Ref can be a branch, commit or tag
func GetContents(ctx context.Context, repo *repo_model.Repository, refCommit *utils.RefCommit, treePath string, forList bool) (*api.ContentsResponse, error) {
	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	commit := refCommit.Commit
	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	refType := refCommit.RefName.RefType()
	if refType != git.RefTypeBranch && refType != git.RefTypeTag && refType != git.RefTypeCommit {
		return nil, fmt.Errorf("no commit found for the ref [ref: %s]", refCommit.RefName)
	}

	selfURL, err := url.Parse(repo.APIURL() + "/contents/" + util.PathEscapeSegments(treePath) + "?ref=" + url.QueryEscape(refCommit.InputRef))
	if err != nil {
		return nil, err
	}
	selfURLString := selfURL.String()

	err = gitRepo.AddLastCommitCache(repo.GetCommitsCountCacheKey(refCommit.InputRef, refType != git.RefTypeCommit), repo.FullName(), refCommit.CommitID)
	if err != nil {
		return nil, err
	}

	lastCommit, err := commit.GetCommitByPath(treePath)
	if err != nil {
		return nil, err
	}

	// All content types have these fields in populated
	contentsResponse := &api.ContentsResponse{
		Name:          entry.Name(),
		Path:          treePath,
		SHA:           entry.ID.String(),
		LastCommitSHA: lastCommit.ID.String(),
		Size:          entry.Size(),
		URL:           &selfURLString,
		Links: &api.FileLinksResponse{
			Self: &selfURLString,
		},
	}

	// GitHub doesn't have these fields in the response, but we could follow other similar APIs to name them
	// https://docs.github.com/en/rest/commits/commits?apiVersion=2022-11-28#list-commits
	if lastCommit.Committer != nil {
		contentsResponse.LastCommitterDate = lastCommit.Committer.When
	}
	if lastCommit.Author != nil {
		contentsResponse.LastAuthorDate = lastCommit.Author.When
	}

	// Now populate the rest of the ContentsResponse based on entry type
	if entry.IsRegular() || entry.IsExecutable() {
		contentsResponse.Type = string(ContentTypeRegular)
		// if it is listing the repo root dir, don't waste system resources on reading content
		if !forList {
			blobResponse, err := GetBlobBySHA(ctx, repo, gitRepo, entry.ID.String())
			if err != nil {
				return nil, err
			}
			contentsResponse.Encoding = blobResponse.Encoding
			contentsResponse.Content = blobResponse.Content
		}
	} else if entry.IsDir() {
		contentsResponse.Type = string(ContentTypeDir)
	} else if entry.IsLink() {
		contentsResponse.Type = string(ContentTypeLink)
		// The target of a symlink file is the content of the file
		targetFromContent, err := entry.Blob().GetBlobContent(1024)
		if err != nil {
			return nil, err
		}
		contentsResponse.Target = &targetFromContent
	} else if entry.IsSubModule() {
		contentsResponse.Type = string(ContentTypeSubmodule)
		submodule, err := commit.GetSubModule(treePath)
		if err != nil {
			return nil, err
		}
		if submodule != nil && submodule.URL != "" {
			contentsResponse.SubmoduleGitURL = &submodule.URL
		}
	}
	// Handle links
	if entry.IsRegular() || entry.IsLink() || entry.IsExecutable() {
		downloadURL, err := url.Parse(repo.HTMLURL() + "/raw/" + refCommit.RefName.RefWebLinkPath() + "/" + util.PathEscapeSegments(treePath))
		if err != nil {
			return nil, err
		}
		downloadURLString := downloadURL.String()
		contentsResponse.DownloadURL = &downloadURLString
	}
	if !entry.IsSubModule() {
		htmlURL, err := url.Parse(repo.HTMLURL() + "/src/" + refCommit.RefName.RefWebLinkPath() + "/" + util.PathEscapeSegments(treePath))
		if err != nil {
			return nil, err
		}
		htmlURLString := htmlURL.String()
		contentsResponse.HTMLURL = &htmlURLString
		contentsResponse.Links.HTMLURL = &htmlURLString

		gitURL, err := url.Parse(repo.APIURL() + "/git/blobs/" + url.PathEscape(entry.ID.String()))
		if err != nil {
			return nil, err
		}
		gitURLString := gitURL.String()
		contentsResponse.GitURL = &gitURLString
		contentsResponse.Links.GitURL = &gitURLString
	}

	return contentsResponse, nil
}

// GetBlobBySHA get the GitBlobResponse of a repository using a sha hash.
func GetBlobBySHA(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, sha string) (*api.GitBlobResponse, error) {
	gitBlob, err := gitRepo.GetBlob(sha)
	if err != nil {
		return nil, err
	}
	ret := &api.GitBlobResponse{
		SHA:  gitBlob.ID.String(),
		URL:  repo.APIURL() + "/git/blobs/" + url.PathEscape(gitBlob.ID.String()),
		Size: gitBlob.Size(),
	}
	if gitBlob.Size() <= setting.API.DefaultMaxBlobSize {
		content, err := gitBlob.GetBlobContentBase64()
		if err != nil {
			return nil, err
		}
		ret.Encoding, ret.Content = util.ToPointer("base64"), &content
	}
	return ret, nil
}
