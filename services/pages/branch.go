// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
)

// UpdateDueBranchSources processes a bounded group of repository directory sources.
func UpdateDueBranchSources(ctx context.Context, limit int) error {
	if limit < 1 || limit > 500 {
		return errors.New("UpdateDueBranchSources: invalid batch size")
	}
	due := make([]*pages_model.Site, 0, limit)
	if err := db.GetEngine(ctx).
		Where("source_type = ? AND deleted = ? AND enabled = ? AND source_next_check_unix <= ?", pages_model.PublishSourceBranch, false, true, timeutil.TimeStampNow()).
		Asc("source_next_check_unix", "repo_id").Limit(limit).Find(&due); err != nil {
		return fmt.Errorf("UpdateDueBranchSources: query: %w", err)
	}
	for _, site := range due {
		if err := updateBranchSource(ctx, site); err != nil {
			log.Warn("Pages branch source for repo %d: %v", site.RepoID, err)
		}
	}
	return nil
}

func updateBranchSource(ctx context.Context, initial *pages_model.Site) error {
	limits := configuredArchiveLimits()
	repo, err := repo_model.GetRepositoryByID(ctx, initial.RepoID)
	if err != nil {
		return setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorRepositoryMissing, 5*time.Minute)
	}
	if repo.IsArchived {
		return setBranchCheckResult(ctx, initial, initial.SourceCommitID, "", time.Minute)
	}
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorGitUnavailable, 5*time.Minute)
	}
	defer gitRepo.Close()
	commitID, err := gitRepo.GetBranchCommitID(ctx, initial.SourceBranch)
	if err != nil {
		return setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorBranchMissing, 5*time.Minute)
	}
	currentVersion, hasCurrentVersion, err := pages_model.GetCurrentVersion(ctx, initial)
	if err != nil {
		return fmt.Errorf("updateBranchSource: current version: %w", err)
	}
	if commitID == initial.SourceCommitID && hasCurrentVersion {
		if _, err := pages_content.NewContentStore().Stat(initial.RepoID, currentVersion.TarGzSHA256); err == nil {
			return setBranchCheckResult(ctx, initial, commitID, "", time.Minute)
		}
	}

	work, cleanup, err := setting.AppDataTempDir("pages").MkdirTempRandom("branch-*")
	if err != nil {
		return fmt.Errorf("updateBranchSource: work directory: %w", err)
	}
	defer cleanup()
	tarPath := filepath.Join(work, "source.tar")
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("updateBranchSource: TAR file: %w", err)
	}
	command := gitcmd.NewCommand("archive").AddOptionFormat("--format=tar").AddDynamicArguments(commitID)
	if initial.SourcePath != "" {
		command.AddDynamicArguments(initial.SourcePath)
	}
	err = command.WithStdoutCopy(&byteLimitWriter{writer: tarFile, remaining: limits.MaxTarBytes}).WithRepo(repo).RunWithStderr(ctx)
	closeErr := tarFile.Close()
	if err != nil || closeErr != nil {
		_ = setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorArchiveFailed, 5*time.Minute)
		return fmt.Errorf("updateBranchSource: git archive: %w", errors.Join(err, closeErr))
	}
	sourceRoot := filepath.Join(work, "source")
	if err := extractGitArchive(tarPath, sourceRoot, initial.SourcePath, limits); err != nil {
		_ = setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorArchiveInvalid, 5*time.Minute)
		return fmt.Errorf("updateBranchSource: extract archive: %w", err)
	}
	candidatePath := filepath.Join(work, "candidate.tar.gz")
	candidate, err := os.Create(candidatePath)
	if err != nil {
		return fmt.Errorf("updateBranchSource: candidate: %w", err)
	}
	err = pages_content.PackDirectory(ctx, sourceRoot, candidate, limits)
	closeErr = candidate.Close()
	if err != nil || closeErr != nil {
		_ = setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorArchiveInvalid, 5*time.Minute)
		return fmt.Errorf("updateBranchSource: pack: %w", errors.Join(err, closeErr))
	}
	input, err := os.Open(candidatePath)
	if err != nil {
		return fmt.Errorf("updateBranchSource: open candidate: %w", err)
	}
	_, err = commitContent(ctx, initial.RepoID, input, contentCommitOptions{
		Origin:         pages_model.VersionOriginBranch,
		RequiredSource: pages_model.PublishSourceBranch,
		ValidateSource: func(ctx context.Context, site *pages_model.Site) error {
			if site.SourceBranch != initial.SourceBranch || site.SourcePath != initial.SourcePath {
				return ErrSourceChanged
			}
			current, err := gitRepo.GetBranchCommitID(ctx, site.SourceBranch)
			if err != nil || current != commitID {
				return ErrSourceChanged
			}
			return nil
		},
	})
	_ = input.Close()
	if err != nil {
		if !errors.Is(err, ErrSourceChanged) {
			_ = setBranchCheckResult(ctx, initial, initial.SourceCommitID, pages_model.SourceErrorCommitFailed, 5*time.Minute)
		}
		return fmt.Errorf("updateBranchSource: commit content: %w", err)
	}
	return setBranchCheckResult(ctx, initial, commitID, "", time.Minute)
}

func setBranchCheckResult(ctx context.Context, initial *pages_model.Site, commitID string, errorCode pages_model.SourceErrorCode, retry time.Duration) error {
	release, err := lockSiteState(ctx, initial.RepoID)
	if err != nil {
		return fmt.Errorf("setBranchCheckResult: lock site: %w", err)
	}
	defer release()
	_, err = db.GetEngine(ctx).
		Where("repo_id = ? AND deleted = ? AND source_type = ? AND source_branch = ? AND source_path = ?", initial.RepoID, false, pages_model.PublishSourceBranch, initial.SourceBranch, initial.SourcePath).
		Cols("source_commit_id", "source_error_code", "source_next_check_unix").
		Update(&pages_model.Site{
			SourceCommitID:      commitID,
			SourceErrorCode:     errorCode,
			SourceNextCheckUnix: int64(timeutil.TimeStampNow().AddDuration(retry)),
		})
	if err != nil {
		return fmt.Errorf("setBranchCheckResult: %w", err)
	}
	return nil
}

type byteLimitWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *byteLimitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, pages_content.ErrArchiveTooLarge
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func extractGitArchive(tarPath, root, sourcePath string, limits pages_content.Limits) error {
	input, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	prefix := strings.Trim(path.Clean(sourcePath), "/")
	if prefix == "." {
		prefix = ""
	}
	reader := tar.NewReader(input)
	entries := 0
	seen := make(map[string]struct{})
	for {
		h, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		entries++
		if entries > limits.MaxEntries {
			return pages_content.ErrArchiveTooLarge
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if err := extractGitArchiveEntry(reader, h, root, prefix, seen, limits); err != nil {
			return err
		}
	}
	return nil
}

func extractGitArchiveEntry(reader *tar.Reader, header *tar.Header, root, prefix string, seen map[string]struct{}, limits pages_content.Limits) error {
	name := strings.TrimSuffix(header.Name, "/")
	if prefix != "" {
		if name == prefix {
			return nil
		}
		name = strings.TrimPrefix(name, prefix+"/")
		if name == header.Name {
			return pages_content.ErrArchivePath
		}
	}
	if name == "" || name == "." {
		return nil
	}
	if path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") ||
		strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return pages_content.ErrArchivePath
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := createSafeParents(root, path.Dir(name)); err != nil {
		return err
	}
	if _, exists := seen[name]; exists {
		return pages_content.ErrArchiveEntry
	}
	seen[name] = struct{}{}
	info, existingErr := os.Lstat(target)
	if existingErr != nil && !os.IsNotExist(existingErr) {
		return existingErr
	}
	switch header.Typeflag {
	case tar.TypeDir:
		if existingErr == nil && !info.IsDir() {
			return pages_content.ErrArchiveEntry
		}
		if os.IsNotExist(existingErr) {
			return os.Mkdir(target, 0o755)
		}
		return nil
	case tar.TypeReg, 0:
		if existingErr == nil {
			return pages_content.ErrArchiveEntry
		}
		if header.Size < 0 || header.Size > limits.MaxFileBytes {
			return pages_content.ErrArchiveTooLarge
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(output, reader, header.Size)
		return errors.Join(copyErr, output.Close())
	case tar.TypeSymlink:
		if existingErr == nil {
			return pages_content.ErrArchiveEntry
		}
		return os.Symlink(header.Linkname, target)
	default:
		return fmt.Errorf("extractGitArchiveEntry: %w: unsupported Git archive entry %q type %d", pages_content.ErrArchiveEntry, name, header.Typeflag)
	}
}

func createSafeParents(root, relative string) error {
	if relative == "." || relative == "" {
		return nil
	}
	current := root
	for part := range strings.SplitSeq(relative, "/") {
		if part == "" || part == "." || part == ".." {
			return pages_content.ErrArchivePath
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if !info.IsDir() {
			return pages_content.ErrArchivePath
		}
	}
	return nil
}
