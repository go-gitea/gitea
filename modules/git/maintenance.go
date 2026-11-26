// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// thresholds and formulas follow Gitaly's housekeeping, the "gcAuto" ones are the defaults of "git gc --auto"
const (
	staleLockAge     = time.Hour
	staleTempFileAge = 24 * time.Hour
	staleTempDirAge  = 72 * time.Hour
	staleKeepAge     = 7 * 24 * time.Hour
	fullRepackPeriod = 5 * 24 * time.Hour
	looseObjectLimit = 1024
	gcAutoLooseLimit = 6700
	gcAutoPackLimit  = 50

	fullRepackTimestampFile = ".gitea-full-repack-timestamp"
)

func MaintainRepository(ctx context.Context, repo RepositoryFacade) (changed bool, _ error) {
	repoPath := gitrepo.RepoLocalPath(repo)
	stats, changed, err := scanRepository(repoPath)
	if err != nil {
		return changed, err
	}

	run := func(cmd *gitcmd.Command) error {
		changed = true
		if err := cmd.WithRepo(repo).RunWithStderr(ctx); err != nil && !gitcmd.IsStderr(err, gitcmd.StderrGcAlreadyRunning) {
			return err
		}
		return nil
	}
	gcArgs := gitcmd.ToTrustedCmdArgs(setting.Git.GCArgs)
	if stats.needsFullRepack() {
		timestampErr := os.WriteFile(filepath.Join(repoPath, fullRepackTimestampFile), nil, 0o644) // before "git gc" runs, so a failing one waits for the next period like in Gitaly
		return true, errors.Join(timestampErr, run(gitcmd.NewCommand("gc").AddArguments(gcArgs...)))
	}
	if !DefaultFeatures().SupportGeometricRepack {
		if stats.needsAutoGc() {
			err = run(gitcmd.NewCommand("gc", "--auto").AddConfig("gc.autoDetach", "false").AddArguments(gcArgs...))
		}
	} else if stats.needsGeometricRepack() {
		if err = run(gitcmd.NewCommand("repack", "-d", "-l", "--geometric=2", "--write-midx", "--no-write-bitmap-index")); err == nil {
			err = run(gitcmd.NewCommand("commit-graph", "write", "--reachable", "--split"))
		}
	}
	if stats.needsRefPacking() {
		err = errors.Join(err, run(gitcmd.NewCommand("pack-refs", "--all")))
	}
	return changed, err
}

type maintenanceStats struct {
	collectablePacks int // neither kept nor cruft packs
	untrackedPacks   int // packs added since the last geometric repack wrote the multi-pack-index
	packsSize        int64
	lastFullRepack   time.Time
	looseObjects     int
	looseRefs        int
	packedRefsSize   int64
}

func scanRepository(repoPath string) (stats maintenanceStats, removed bool, _ error) {
	var errs []error
	failed := func(err error) bool {
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
		return err != nil
	}
	removeIfOlder := func(path string, info fs.FileInfo, maxAge time.Duration, remove func(string) error) {
		if time.Since(info.ModTime()) >= maxAge && !failed(remove(path)) {
			log.Warn("Removed %s left behind by an interrupted git process", path)
			removed = true
		}
	}
	removeFileIfOlder := func(path string, maxAge time.Duration) {
		if info, err := os.Lstat(path); !failed(err) && info.Mode().IsRegular() {
			removeIfOlder(path, info, maxAge, os.Remove)
		}
	}

	for _, name := range []string{"HEAD.lock", "config.lock", "packed-refs.lock", "packed-refs.new", "objects/info/commit-graph.lock", "objects/info/commit-graphs/commit-graph-chain.lock", "objects/pack/multi-pack-index.lock", "reftable/tables.list.lock"} {
		removeFileIfOlder(filepath.Join(repoPath, name), staleLockAge)
	}
	if info, err := os.Lstat(filepath.Join(repoPath, "packed-refs")); !failed(err) {
		stats.packedRefsSize = info.Size()
	}
	_ = filepath.WalkDir(filepath.Join(repoPath, "refs"), func(path string, entry fs.DirEntry, err error) error {
		switch {
		case failed(err), !entry.Type().IsRegular():
		case strings.HasSuffix(entry.Name(), ".lock"):
			removeFileIfOlder(path, staleLockAge)
		default:
			stats.looseRefs++
		}
		return nil
	})
	if info, err := os.Lstat(filepath.Join(repoPath, fullRepackTimestampFile)); !failed(err) {
		stats.lastFullRepack = info.ModTime()
	}

	objectsPath := filepath.Join(repoPath, "objects")
	objectEntries, err := os.ReadDir(objectsPath)
	failed(err)
	for _, entry := range objectEntries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "tmp_objdir") {
			continue
		}
		if info, err := entry.Info(); !failed(err) {
			removeIfOlder(filepath.Join(objectsPath, entry.Name()), info, staleTempDirAge, os.RemoveAll)
		}
	}
	looseEntries, err := os.ReadDir(filepath.Join(objectsPath, "17")) // like "git gc --auto", estimate from one of the 256 fan-out directories
	failed(err)
	for _, entry := range looseEntries {
		if len(entry.Name()) == Sha1ObjectFormat.FullLength()-2 || len(entry.Name()) == Sha256ObjectFormat.FullLength()-2 {
			stats.looseObjects += 256
		}
	}

	removeTempFiles := func(dir string) []fs.DirEntry {
		entries, err := os.ReadDir(dir)
		failed(err)
		for _, entry := range entries {
			if entry.Type().IsRegular() && (strings.HasPrefix(entry.Name(), "tmp_") || strings.HasPrefix(entry.Name(), ".tmp-")) {
				removeFileIfOlder(filepath.Join(dir, entry.Name()), staleTempFileAge)
			}
		}
		return entries
	}
	removeTempFiles(filepath.Join(objectsPath, "info", "commit-graphs"))

	packPath := filepath.Join(objectsPath, "pack")
	packEntries := removeTempFiles(packPath)
	packFiles := make(map[string]bool, len(packEntries))
	for _, entry := range packEntries {
		name := entry.Name()
		packFiles[name] = true
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, "pack-") || !strings.HasSuffix(name, ".keep") {
			continue
		}
		path := filepath.Join(packPath, name)
		if info, err := entry.Info(); !failed(err) && time.Since(info.ModTime()) >= staleKeepAge {
			if isTransferKeep, err := isTransferKeepFile(path); !failed(err) && isTransferKeep {
				removeIfOlder(path, info, staleKeepAge, os.Remove)
			}
		}
	}
	for _, entry := range packEntries {
		name, ok := strings.CutSuffix(entry.Name(), ".pack")
		if !ok || strings.HasPrefix(name, ".") || !packFiles[name+".idx"] {
			continue
		}
		if info, err := entry.Info(); !failed(err) {
			stats.untrackedPacks++
			stats.packsSize += info.Size()
			if !packFiles[name+".keep"] && !packFiles[name+".mtimes"] {
				stats.collectablePacks++
			}
		}
	}
	if packFiles["multi-pack-index"] {
		trackedPacks, err := readMultiPackIndexPackCount(filepath.Join(packPath, "multi-pack-index"))
		failed(err)
		stats.untrackedPacks -= trackedPacks
	}
	return stats, removed, errors.Join(errs...)
}

// other ".keep" files protect their packs on purpose
func isTransferKeepFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	prefix, err := util.ReadWithLimit(file, len("receive-pack "))
	return bytes.HasPrefix(prefix, []byte("receive-pack ")) || bytes.HasPrefix(prefix, []byte("fetch-pack ")), err
}

func readMultiPackIndexPackCount(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	header, err := util.ReadWithLimit(file, 12)
	if err != nil || len(header) < 12 || string(header[:4]) != "MIDX" {
		return 0, err
	}
	return int(binary.BigEndian.Uint32(header[8:])), nil
}

func (stats maintenanceStats) needsFullRepack() bool {
	return (stats.collectablePacks > 1 || stats.looseObjects > looseObjectLimit) && time.Since(stats.lastFullRepack) > fullRepackPeriod
}

func (stats maintenanceStats) needsAutoGc() bool {
	return stats.looseObjects > gcAutoLooseLimit || stats.collectablePacks > gcAutoPackLimit
}

func (stats maintenanceStats) needsGeometricRepack() bool {
	return stats.looseObjects > looseObjectLimit || float64(stats.untrackedPacks) > max(2, math.Log(float64(stats.packsSize)/1024/1024)/math.Log(1.8))
}

func (stats maintenanceStats) needsRefPacking() bool {
	return float64(stats.looseRefs) > max(16, math.Log(float64(stats.packedRefsSize)/100)/math.Log(1.15))
}
