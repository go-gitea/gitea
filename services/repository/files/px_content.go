package files

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
)

type checkOption func(*api.CommitContentsResponse, *git.TreeEntry) error

// GetCommitContentsOrList gets the meta data of a file's contents (*ContentsResponse) if treePath not a tree
// directory, otherwise a listing of file contents ([]*ContentsResponse). Ref can be a branch, commit or tag
func GetCommitContentsOrList(ctx context.Context, repo *repo_model.Repository, treePath, ref string) (any, error) {
	if repo.IsEmpty {
		return make([]any, 0), nil
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	if entry.Type() != "tree" {
		return GetCommitContents(ctx, repo, treePath, origRef, false, checkIsNonText)
	}

	// We are in a directory, so we return a list of FileContentResponse objects
	var fileList []*api.CommitContentsResponse

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
		fileContentResponse, err := GetCommitContents(ctx, repo, subTreePath, origRef, true)
		if err != nil {
			return nil, err
		}
		fileList = append(fileList, fileContentResponse)
	}
	return fileList, nil
}

// GetCommitContents gets the meta data on a directory's or a file's contents. Ref can be a branch, commit or tag
func GetCommitContents(
	ctx context.Context,
	repo *repo_model.Repository,
	treePath,
	ref string,
	forList bool,
	options ...checkOption,
) (*api.CommitContentsResponse, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}
	commitID := commit.ID.String()
	if len(ref) >= 4 && strings.HasPrefix(commitID, ref) {
		ref = commit.ID.String()
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	refType := gitRepo.GetRefType(ref)
	if refType == "invalid" {
		return nil, fmt.Errorf("no commit found for the ref [ref: %s]", ref)
	}

	selfURL, err := url.Parse(repo.APIURL() + "/contents/" + util.PathEscapeSegments(treePath) + "?ref=" + url.QueryEscape(origRef))
	if err != nil {
		return nil, err
	}
	selfURLString := selfURL.String()

	err = gitRepo.AddLastCommitCache(repo.GetCommitsCountCacheKey(ref, refType != git.ObjectCommit), repo.FullName(), commitID)
	if err != nil {
		return nil, err
	}

	lastCommit, err := commit.GetCommitByPath(treePath)
	if err != nil {
		return nil, err
	}

	// All content types have these fields in populated
	contentsResponse := &api.CommitContentsResponse{
		Name:              entry.Name(),
		Path:              treePath,
		SHA:               entry.ID.String(),
		LastCommitSHA:     lastCommit.ID.String(),
		LastCommitMessage: lastCommit.CommitMessage,
		LastCommitCreate:  lastCommit.Committer.When,
		Size:              entry.Size(),
		URL:               &selfURLString,
		Links: &api.FileLinksResponse{
			Self: &selfURLString,
		},
	}

	for _, option := range options {
		if err = option(contentsResponse, entry); err != nil {
			return nil, err
		}
	}

	if p, b := isLFS(entry); b {
		contentsResponse.Size = p.Size
		contentsResponse.IsLFS = true
	}

	// Now populate the rest of the ContentsResponse based on entry type
	if entry.IsRegular() || entry.IsExecutable() {
		contentsResponse.Type = string(ContentTypeRegular)
		if blobResponse, err := GetBlobBySHA(ctx, repo, gitRepo, entry.ID.String()); err != nil {
			return nil, err
		} else if !forList {
			// We don't show the content if we are getting a list of FileContentResponses
			contentsResponse.Encoding = &blobResponse.Encoding
			contentsResponse.Content = &blobResponse.Content
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
	if entry.IsRegular() || entry.IsLink() {
		downloadURL, err := url.Parse(repo.HTMLURL() + "/raw/" + url.PathEscape(string(refType)) + "/" + util.PathEscapeSegments(ref) + "/" + util.PathEscapeSegments(treePath))
		if err != nil {
			return nil, err
		}
		downloadURLString := downloadURL.String()
		contentsResponse.DownloadURL = &downloadURLString
	}
	if !entry.IsSubModule() {
		htmlURL, err := url.Parse(repo.HTMLURL() + "/src/" + url.PathEscape(string(refType)) + "/" + util.PathEscapeSegments(ref) + "/" + util.PathEscapeSegments(treePath))
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

func checkIsNonText(response *api.CommitContentsResponse, entry *git.TreeEntry) error {
	isNonText, err := isNonText(entry)

	if err != nil {
		return err
	}

	response.IsNonText = &isNonText

	return nil
}

func isLFS(entry *git.TreeEntry) (lfs.Pointer, bool) {
	if !entry.IsRegular() || entry.Size() > 512 {
		return lfs.Pointer{}, false
	}

	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return lfs.Pointer{}, false
	}
	defer reader.Close()

	p, err := lfs.ReadPointer(reader)

	return p, err == nil
}

func isNonText(entry *git.TreeEntry) (bool, error) {
	if !entry.IsRegular() {
		return false, nil
	}

	dataRc, err := entry.Blob().DataAsync()
	if err != nil {
		return false, err
	}

	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	st := typesniffer.DetectContentType(buf)

	return !st.IsText(), nil
}
