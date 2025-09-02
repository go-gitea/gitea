// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"io"
	"net/url"
	"path"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
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

type GetContentsOrListOptions struct {
	TreePath                 string
	IncludeSingleFileContent bool // include the file's content when the tree path is a file
	IncludeLfsMetadata       bool
	IncludeCommitMetadata    bool
	IncludeCommitMessage     bool
}

// GetContentsOrList gets the metadata of a file's contents (*ContentsResponse) if treePath not a tree
// directory, otherwise a listing of file contents ([]*ContentsResponse). Ref can be a branch, commit or tag
func GetContentsOrList(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, refCommit *utils.RefCommit, opts GetContentsOrListOptions) (ret api.ContentsExtResponse, _ error) {
	entry, err := prepareGetContentsEntry(refCommit, &opts.TreePath)
	if repo.IsEmpty && opts.TreePath == "" {
		return api.ContentsExtResponse{DirContents: make([]*api.ContentsResponse, 0)}, nil
	}
	if err != nil {
		return ret, err
	}

	// get file contents
	if entry.Type() != "tree" {
		ret.FileContents, err = getFileContentsByEntryInternal(ctx, repo, gitRepo, refCommit, entry, opts)
		return ret, err
	}

	// list directory contents
	gitTree, err := refCommit.Commit.SubTree(opts.TreePath)
	if err != nil {
		return ret, err
	}
	entries, err := gitTree.ListEntries()
	if err != nil {
		return ret, err
	}
	ret.DirContents = make([]*api.ContentsResponse, 0, len(entries))
	for _, e := range entries {
		subOpts := opts
		subOpts.TreePath = path.Join(opts.TreePath, e.Name())
		subOpts.IncludeSingleFileContent = false // never include file content when listing a directory
		fileContentResponse, err := GetFileContents(ctx, repo, gitRepo, refCommit, subOpts)
		if err != nil {
			return ret, err
		}
		ret.DirContents = append(ret.DirContents, fileContentResponse)
	}
	return ret, nil
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

func prepareGetContentsEntry(refCommit *utils.RefCommit, treePath *string) (*git.TreeEntry, error) {
	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanGitTreePath(*treePath)
	if cleanTreePath == "" && *treePath != "" {
		return nil, ErrFilenameInvalid{Path: *treePath}
	}
	*treePath = cleanTreePath

	// Only allow safe ref types
	refType := refCommit.RefName.RefType()
	if refType != git.RefTypeBranch && refType != git.RefTypeTag && refType != git.RefTypeCommit {
		return nil, util.NewNotExistErrorf("no commit found for the ref [ref: %s]", refCommit.RefName)
	}

	return refCommit.Commit.GetTreeEntryByPath(*treePath)
}

// GetFileContents gets the metadata on a file's contents. Ref can be a branch, commit or tag
func GetFileContents(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, refCommit *utils.RefCommit, opts GetContentsOrListOptions) (*api.ContentsResponse, error) {
	entry, err := prepareGetContentsEntry(refCommit, &opts.TreePath)
	if err != nil {
		return nil, err
	}
	return getFileContentsByEntryInternal(ctx, repo, gitRepo, refCommit, entry, opts)
}

func getFileContentsByEntryInternal(_ context.Context, repo *repo_model.Repository, gitRepo *git.Repository, refCommit *utils.RefCommit, entry *git.TreeEntry, opts GetContentsOrListOptions) (*api.ContentsResponse, error) {
	refType := refCommit.RefName.RefType()
	commit := refCommit.Commit
	selfURL, err := url.Parse(repo.APIURL() + "/contents/" + util.PathEscapeSegments(opts.TreePath) + "?ref=" + url.QueryEscape(refCommit.InputRef))
	if err != nil {
		return nil, err
	}
	selfURLString := selfURL.String()

	// All content types have these fields in populated
	contentsResponse := &api.ContentsResponse{
		Name: entry.Name(),
		Path: opts.TreePath,
		SHA:  entry.ID.String(),
		Size: entry.Size(),
		URL:  &selfURLString,
		Links: &api.FileLinksResponse{
			Self: &selfURLString,
		},
	}

	if opts.IncludeCommitMetadata || opts.IncludeCommitMessage {
		err = gitRepo.AddLastCommitCache(repo.GetCommitsCountCacheKey(refCommit.InputRef, refType != git.RefTypeCommit), repo.FullName(), refCommit.CommitID)
		if err != nil {
			return nil, err
		}

		lastCommit, err := refCommit.Commit.GetCommitByPath(opts.TreePath)
		if err != nil {
			return nil, err
		}

		if opts.IncludeCommitMetadata {
			contentsResponse.LastCommitSHA = util.ToPointer(lastCommit.ID.String())
			// GitHub doesn't have these fields in the response, but we could follow other similar APIs to name them
			// https://docs.github.com/en/rest/commits/commits?apiVersion=2022-11-28#list-commits
			if lastCommit.Committer != nil {
				contentsResponse.LastCommitterDate = util.ToPointer(lastCommit.Committer.When)
			}
			if lastCommit.Author != nil {
				contentsResponse.LastAuthorDate = util.ToPointer(lastCommit.Author.When)
			}
		}
		if opts.IncludeCommitMessage {
			contentsResponse.LastCommitMessage = util.ToPointer(lastCommit.Message())
		}
	}

	// Now populate the rest of the ContentsResponse based on the entry type
	if entry.IsRegular() || entry.IsExecutable() {
		contentsResponse.Type = string(ContentTypeRegular)
		// if it is listing the repo root dir, don't waste system resources on reading content
		if opts.IncludeSingleFileContent {
			blobResponse, err := GetBlobBySHA(repo, gitRepo, entry.ID.String())
			if err != nil {
				return nil, err
			}
			contentsResponse.Encoding, contentsResponse.Content = blobResponse.Encoding, blobResponse.Content
			contentsResponse.LfsOid, contentsResponse.LfsSize = blobResponse.LfsOid, blobResponse.LfsSize
		} else if opts.IncludeLfsMetadata {
			contentsResponse.LfsOid, contentsResponse.LfsSize, err = parsePossibleLfsPointerBlob(gitRepo, entry.ID.String())
			if err != nil {
				return nil, err
			}
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
		submodule, err := commit.GetSubModule(opts.TreePath)
		if err != nil {
			return nil, err
		}
		if submodule != nil && submodule.URL != "" {
			contentsResponse.SubmoduleGitURL = &submodule.URL
		}
	}
	// Handle links
	if entry.IsRegular() || entry.IsLink() || entry.IsExecutable() {
		downloadURL, err := url.Parse(repo.HTMLURL() + "/raw/" + refCommit.RefName.RefWebLinkPath() + "/" + util.PathEscapeSegments(opts.TreePath))
		if err != nil {
			return nil, err
		}
		downloadURLString := downloadURL.String()
		contentsResponse.DownloadURL = &downloadURLString
	}
	if !entry.IsSubModule() {
		htmlURL, err := url.Parse(repo.HTMLURL() + "/src/" + refCommit.RefName.RefWebLinkPath() + "/" + util.PathEscapeSegments(opts.TreePath))
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

func GetBlobBySHA(repo *repo_model.Repository, gitRepo *git.Repository, sha string) (*api.GitBlobResponse, error) {
	gitBlob, err := gitRepo.GetBlob(sha)
	if err != nil {
		return nil, err
	}
	ret := &api.GitBlobResponse{
		SHA:  gitBlob.ID.String(),
		URL:  repo.APIURL() + "/git/blobs/" + url.PathEscape(gitBlob.ID.String()),
		Size: gitBlob.Size(),
	}

	blobSize := gitBlob.Size()
	if blobSize > setting.API.DefaultMaxBlobSize {
		return ret, nil
	}

	var originContent *strings.Builder
	if 0 < blobSize && blobSize < lfs.MetaFileMaxSize {
		originContent = &strings.Builder{}
	}

	content, err := gitBlob.GetBlobContentBase64(originContent)
	if err != nil {
		return nil, err
	}

	ret.Encoding, ret.Content = util.ToPointer("base64"), &content
	if originContent != nil {
		ret.LfsOid, ret.LfsSize = parsePossibleLfsPointerBuffer(strings.NewReader(originContent.String()))
	}
	return ret, nil
}

func parsePossibleLfsPointerBuffer(r io.Reader) (*string, *int64) {
	p, _ := lfs.ReadPointer(r)
	if p.IsValid() {
		return &p.Oid, &p.Size
	}
	return nil, nil
}

func parsePossibleLfsPointerBlob(gitRepo *git.Repository, sha string) (*string, *int64, error) {
	gitBlob, err := gitRepo.GetBlob(sha)
	if err != nil {
		return nil, nil, err
	}
	if gitBlob.Size() > lfs.MetaFileMaxSize {
		return nil, nil, nil // not a LFS pointer
	}
	buf, err := gitBlob.GetBlobContent(lfs.MetaFileMaxSize)
	if err != nil {
		return nil, nil, err
	}
	oid, size := parsePossibleLfsPointerBuffer(strings.NewReader(buf))
	return oid, size, nil
}
