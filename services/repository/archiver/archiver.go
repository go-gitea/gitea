// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package archiver

import (
	"context"
	"fmt"
	"io"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/httpcache"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	gitea_context "gitea.dev/services/context"
)

// ArchiveRequest defines the parameters of an archive request, which notably
// includes the specific repository being archived as well as the commit, the
// name by which it was requested, and the kind of archive being requested.
// This is entirely opaque to external entities, though, and mostly used as a
// handle elsewhere.
type ArchiveRequest struct {
	Repo     *repo_model.Repository
	Type     repo_model.ArchiveType
	CommitID string
	Paths    []string

	archiveRefShortName string // the ref short name to download the archive, for example: "master", "v1.0.0", "commit id"
}

// NewRequest creates an archival request, based on the URI.
func NewRequest(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, archiveRefExt string, paths []string) (*ArchiveRequest, error) {
	// here the archiveRefShortName is not a clear ref, it could be a tag, branch or commit id
	archiveRefShortName, archiveType := repo_model.SplitArchiveNameType(archiveRefExt)
	if archiveType == repo_model.ArchiveUnknown {
		return nil, util.NewInvalidArgumentErrorf("unknown format: %s", archiveRefExt)
	}
	if archiveType == repo_model.ArchiveBundle && len(paths) != 0 {
		return nil, util.NewInvalidArgumentErrorf("cannot specify paths when requesting a bundle")
	}

	// Get corresponding commit.
	commit, err := gitRepo.GetCommit(ctx, archiveRefShortName)
	if err != nil {
		return nil, util.NewNotExistErrorf("unrecognized repository reference: %s", archiveRefShortName)
	}

	r := &ArchiveRequest{Repo: repo, archiveRefShortName: archiveRefShortName, Type: archiveType, Paths: paths}
	r.CommitID = commit.ID.String()
	return r, nil
}

// GetArchiveName returns the name of the caller, based on the ref used by the
// caller to create this request.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return strings.ReplaceAll(aReq.archiveRefShortName, "/", "-") + "." + aReq.Type.String()
}

// Stream satisfies the ArchiveRequest being passed in.  Processing
// will occur directly in this routine.
func (aReq *ArchiveRequest) Stream(ctx context.Context, w io.Writer) error {
	if aReq.Type == repo_model.ArchiveBundle {
		return git.CreateBundle(
			ctx,
			aReq.Repo,
			aReq.CommitID,
			w,
		)
	}
	return git.CreateArchive(
		ctx,
		aReq.Repo,
		aReq.Repo.Name,
		aReq.Type.String(),
		w,
		aReq.CommitID,
		aReq.Paths,
	)
}

func ServeRepoArchive(ctx *gitea_context.Base, archiveReq *ArchiveRequest) error {
	// Add nix format link header so tarballs lock correctly:
	// https://github.com/nixos/nix/blob/56763ff918eb308db23080e560ed2ea3e00c80a7/doc/manual/src/protocols/tarball-fetcher.md
	ctx.Resp.Header().Add("Link", fmt.Sprintf(`<%s/archive/%s.%s?rev=%s>; rel="immutable"`,
		archiveReq.Repo.APIURL(),
		archiveReq.CommitID,
		archiveReq.Type.String(),
		archiveReq.CommitID,
	))
	downloadName := archiveReq.Repo.Name + "-" + archiveReq.GetArchiveName()

	// Use weak ETag because the bytes also depend on the git version, compression level and repo config
	etag := fmt.Sprintf(`W/"%s-%s-%t"`, archiveReq.CommitID, archiveReq.Type.String(), setting.Repository.PrefixArchiveFiles)
	if len(archiveReq.Paths) == 0 && httpcache.HandleGenericETagPrivateCache(ctx.Req, ctx.Resp, etag, nil) {
		return nil
	}
	// the header must be set before starting streaming even an error would occur,
	// because errors may happen in git command and such cases aren't in our control.
	httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{Filename: downloadName})
	if err := archiveReq.Stream(ctx, ctx.Resp); err != nil && !ctx.Written() {
		if gitcmd.IsStderr(err, gitcmd.StderrPathSpec) || gitcmd.IsStderr(err, gitcmd.StderrNotTreeObject) {
			return util.NewInvalidArgumentErrorf("path doesn't exist or is invalid")
		}
		return fmt.Errorf("ServeRepoArchive: archive repo %s: failed to stream: %w", archiveReq.Repo.FullName(), err)
	}
	return nil
}
