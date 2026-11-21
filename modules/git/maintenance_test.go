// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveStaleFiles(t *testing.T) {
	repoPath := t.TempDir()
	writeFile := func(name, content string, age time.Duration) string {
		path := filepath.Join(repoPath, name)
		require.NoError(t, util.WriteRegularPathFile(repoPath, name, []byte(content), 0o755, 0o644))
		mtime := time.Now().Add(-age)
		require.NoError(t, os.Chtimes(path, mtime, mtime))
		return path
	}
	day := 24 * time.Hour
	quarantinePath := filepath.Join(repoPath, "objects/tmp_objdir-incoming-a")
	require.NoError(t, os.MkdirAll(filepath.Join(quarantinePath, "pack"), 0o755))
	require.NoError(t, os.Chtimes(quarantinePath, time.Now().Add(-4*day), time.Now().Add(-4*day)))
	stale := []string{
		writeFile("HEAD.lock", "", 2*time.Hour),
		writeFile("packed-refs.new", "", 2*time.Hour),
		writeFile("objects/info/commit-graphs/commit-graph-chain.lock", "", 2*time.Hour),
		writeFile("reftable/tables.list.lock", "", 2*time.Hour),
		writeFile("refs/heads/feature/branch.lock", "", 2*time.Hour),
		writeFile("objects/pack/tmp_pack_b", "", 2*day),
		writeFile("objects/info/commit-graphs/tmp_graph_a", "", 2*day),
		writeFile("objects/pack/.tmp-1-pack-c.pack", "", 2*day),
		writeFile("objects/pack/pack-a.keep", "receive-pack 1 on host\n", 8*day),
		writeFile("objects/pack/pack-b.keep", "fetch-pack 1 on host\n", 8*day),
		quarantinePath,
	}
	kept := []string{
		writeFile("config.lock", "", 0),
		writeFile("index.lock", "", 2*time.Hour),
		writeFile("refs/heads/main", "", 2*time.Hour),
		writeFile("objects/pack/tmp_pack_d", "", time.Hour),
		writeFile("objects/tmp_objdir-incoming-b/pack/tmp_pack_e", "", 4*day),
		writeFile("objects/pack/pack-c.keep", "receive-pack 1 on host\n", day),
		writeFile("objects/pack/pack-d.keep", "", 8*day),
	}

	_, removed, err := scanRepository(repoPath)
	require.NoError(t, err)
	assert.True(t, removed)
	for _, path := range stale {
		assert.NoFileExists(t, path)
		assert.NoDirExists(t, path)
	}
	for _, path := range kept {
		assert.FileExists(t, path)
	}
}

func TestMaintainRepository(t *testing.T) {
	defer test.MockVariableValue(&DefaultFeatures().SupportGeometricRepack, true)()
	repoPath := t.TempDir()
	repo := mockRepository(repoPath)
	require.NoError(t, InitRepository(t.Context(), repo, "sha1", ""))
	importPack := func(prefix string, count int) {
		var stdin strings.Builder
		for i := range count {
			ref := fmt.Sprintf("refs/heads/%s%d", prefix, i)
			_, _ = fmt.Fprintf(&stdin, "commit %s\ncommitter a <a@a> 0 +0000\ndata %d\n%s\n", ref, len(ref), ref)
		}
		require.NoError(t, gitcmd.NewCommand("fast-import").AddConfig("fastimport.unpackLimit", "0").WithDir(repoPath).WithStdinBytes([]byte(stdin.String())).Run(t.Context()))
	}
	maintain := func() maintenanceStats {
		changed, err := MaintainRepository(t.Context(), repo)
		require.NoError(t, err)
		assert.True(t, changed)
		stats, _, err := scanRepository(repoPath)
		require.NoError(t, err)
		return stats
	}
	importPack("a", 1)
	importPack("b", 1)
	require.NoError(t, util.WriteRegularPathFile(repoPath, "objects/17/tmp_obj_a", nil, 0o755, 0o644))
	stats := maintain()
	assert.Equal(t, 1, stats.collectablePacks)
	assert.Zero(t, stats.looseObjects)
	assert.Zero(t, stats.looseRefs)
	assert.WithinDuration(t, time.Now(), stats.lastFullRepack, time.Minute)

	importPack("c", 1)
	importPack("d", 1)
	stats = maintain()
	assert.Zero(t, stats.untrackedPacks)
	assert.FileExists(t, filepath.Join(repoPath, "objects/info/commit-graphs/commit-graph-chain"))

	importPack("r", 20)
	assert.Zero(t, maintain().looseRefs)

	mtime := time.Now().Add(-fullRepackPeriod - time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(repoPath, fullRepackTimestampFile), mtime, mtime))
	assert.Equal(t, 1, maintain().collectablePacks)
}

func TestMaintenanceStatsThresholds(t *testing.T) {
	const mib = 1024 * 1024
	assert.False(t, maintenanceStats{untrackedPacks: 2, packsSize: mib}.needsGeometricRepack())
	assert.True(t, maintenanceStats{untrackedPacks: 3, packsSize: mib}.needsGeometricRepack())
	assert.False(t, maintenanceStats{untrackedPacks: 7, packsSize: 100 * mib}.needsGeometricRepack())
	assert.True(t, maintenanceStats{untrackedPacks: 8, packsSize: 100 * mib}.needsGeometricRepack())
	assert.True(t, maintenanceStats{untrackedPacks: 1, looseObjects: 1280}.needsGeometricRepack())
	assert.False(t, maintenanceStats{looseRefs: 16}.needsRefPacking())
	assert.True(t, maintenanceStats{looseRefs: 17}.needsRefPacking())
	assert.False(t, maintenanceStats{looseRefs: 66, packedRefsSize: mib}.needsRefPacking())
	assert.True(t, maintenanceStats{looseRefs: 67, packedRefsSize: mib}.needsRefPacking())
	assert.False(t, maintenanceStats{collectablePacks: 1}.needsFullRepack())
	assert.True(t, maintenanceStats{collectablePacks: 2}.needsFullRepack())
	assert.False(t, maintenanceStats{collectablePacks: 2, lastFullRepack: time.Now()}.needsFullRepack())
	assert.False(t, maintenanceStats{collectablePacks: 50, looseObjects: 6656}.needsAutoGc())
	assert.True(t, maintenanceStats{collectablePacks: 51}.needsAutoGc())
	assert.True(t, maintenanceStats{looseObjects: 6912}.needsAutoGc())
	assert.True(t, maintenanceStats{looseObjects: 1280}.needsFullRepack())
}
