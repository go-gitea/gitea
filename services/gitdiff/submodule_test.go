// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"context"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestParseSubmoduleInfo(t *testing.T) {
	type testcase struct {
		name    string
		gitdiff string
		infos   map[int]SubmoduleDiffInfo
	}

	tests := []testcase{
		{
			name: "added",
			gitdiff: `diff --git a/.gitmodules b/.gitmodules
new file mode 100644
index 0000000..4ac13c1
--- /dev/null
+++ b/.gitmodules
@@ -0,0 +1,3 @@
+[submodule "gitea-mirror"]
+	path = gitea-mirror
+	url = https://gitea.com/gitea/gitea-mirror
diff --git a/gitea-mirror b/gitea-mirror
new file mode 160000
index 0000000..68972a9
--- /dev/null
+++ b/gitea-mirror
@@ -0,0 +1 @@
+Subproject commit 68972a994719ae5c74e28d8fa82fa27c23399bc8
`,
			infos: map[int]SubmoduleDiffInfo{
				1: {NewRefID: "68972a994719ae5c74e28d8fa82fa27c23399bc8"},
			},
		},
		{
			name: "updated",
			gitdiff: `diff --git a/gitea-mirror b/gitea-mirror
index 68972a9..c8ffe77 160000
--- a/gitea-mirror
+++ b/gitea-mirror
@@ -1 +1 @@
-Subproject commit 68972a994719ae5c74e28d8fa82fa27c23399bc8
+Subproject commit c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d
`,
			infos: map[int]SubmoduleDiffInfo{
				0: {
					PreviousRefID: "68972a994719ae5c74e28d8fa82fa27c23399bc8",
					NewRefID:      "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
				},
			},
		},
		{
			name: "rename",
			gitdiff: `diff --git a/.gitmodules b/.gitmodules
index 4ac13c1..0510edd 100644
--- a/.gitmodules
+++ b/.gitmodules
@@ -1,3 +1,3 @@
 [submodule "gitea-mirror"]
-	path = gitea-mirror
+	path = gitea
 	url = https://gitea.com/gitea/gitea-mirror
diff --git a/gitea-mirror b/gitea
similarity index 100%
rename from gitea-mirror
rename to gitea
`,
		},
		{
			name: "deleted",
			gitdiff: `diff --git a/.gitmodules b/.gitmodules
index 0510edd..e69de29 100644
--- a/.gitmodules
+++ b/.gitmodules
@@ -1,3 +0,0 @@
-[submodule "gitea-mirror"]
-	path = gitea
-	url = https://gitea.com/gitea/gitea-mirror
diff --git a/gitea b/gitea
deleted file mode 160000
index c8ffe77..0000000
--- a/gitea
+++ /dev/null
@@ -1 +0,0 @@
-Subproject commit c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d
`,
			infos: map[int]SubmoduleDiffInfo{
				1: {
					PreviousRefID: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
				},
			},
		},
		{
			name: "moved and updated",
			gitdiff: `diff --git a/.gitmodules b/.gitmodules
index 0510edd..bced3d8 100644
--- a/.gitmodules
+++ b/.gitmodules
@@ -1,3 +1,3 @@
 [submodule "gitea-mirror"]
-	path = gitea
+	path = gitea-1.22
 	url = https://gitea.com/gitea/gitea-mirror
diff --git a/gitea b/gitea
deleted file mode 160000
index c8ffe77..0000000
--- a/gitea
+++ /dev/null
@@ -1 +0,0 @@
-Subproject commit c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d
diff --git a/gitea-1.22 b/gitea-1.22
new file mode 160000
index 0000000..8eefa1f
--- /dev/null
+++ b/gitea-1.22
@@ -0,0 +1 @@
+Subproject commit 8eefa1f6dedf2488db2c9e12c916e8e51f673160
`,
			infos: map[int]SubmoduleDiffInfo{
				1: {
					PreviousRefID: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
				},
				2: {
					NewRefID: "8eefa1f6dedf2488db2c9e12c916e8e51f673160",
				},
			},
		},
		{
			name: "converted to file",
			gitdiff: `diff --git a/.gitmodules b/.gitmodules
index 0510edd..e69de29 100644
--- a/.gitmodules
+++ b/.gitmodules
@@ -1,3 +0,0 @@
-[submodule "gitea-mirror"]
-	path = gitea
-	url = https://gitea.com/gitea/gitea-mirror
diff --git a/gitea b/gitea
deleted file mode 160000
index c8ffe77..0000000
--- a/gitea
+++ /dev/null
@@ -1 +0,0 @@
-Subproject commit c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d
diff --git a/gitea b/gitea
new file mode 100644
index 0000000..33a9488
--- /dev/null
+++ b/gitea
@@ -0,0 +1 @@
+example
`,
			infos: map[int]SubmoduleDiffInfo{
				1: {
					PreviousRefID: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
				},
			},
		},
		{
			name: "converted to submodule",
			gitdiff: `diff --git a/.gitmodules b/.gitmodules
index e69de29..14ee267 100644
--- a/.gitmodules
+++ b/.gitmodules
@@ -0,0 +1,3 @@
+[submodule "gitea"]
+	path = gitea
+	url = https://gitea.com/gitea/gitea-mirror
diff --git a/gitea b/gitea
deleted file mode 100644
index 33a9488..0000000
--- a/gitea
+++ /dev/null
@@ -1 +0,0 @@
-example
diff --git a/gitea b/gitea
new file mode 160000
index 0000000..68972a9
--- /dev/null
+++ b/gitea
@@ -0,0 +1 @@
+Subproject commit 68972a994719ae5c74e28d8fa82fa27c23399bc8
`,
			infos: map[int]SubmoduleDiffInfo{
				2: {
					NewRefID: "68972a994719ae5c74e28d8fa82fa27c23399bc8",
				},
			},
		},
	}

	for _, testcase := range tests {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			diff, err := ParsePatch(db.DefaultContext, setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), "")
			assert.NoError(t, err)

			for i, expected := range testcase.infos {
				actual := diff.Files[i]
				assert.NotNil(t, actual)
				assert.Equal(t, expected, *actual.SubmoduleDiffInfo)
			}
		})
	}
}

func TestSubmoduleInfo(t *testing.T) {
	sdi := &SubmoduleDiffInfo{
		SubmoduleName: "name",
		PreviousRefID: "aaaa",
		NewRefID:      "bbbb",
	}
	ctx := context.Background()
	assert.EqualValues(t, "1111", sdi.CommitRefIDLinkHTML(ctx, "1111"))
	assert.EqualValues(t, "aaaa...bbbb", sdi.CompareRefIDLinkHTML(ctx))
	assert.EqualValues(t, "name", sdi.SubmoduleRepoLinkHTML(ctx))

	sdi.SubmoduleFile = git.NewCommitSubmoduleFile("https://github.com/owner/repo", "1234")
	assert.EqualValues(t, `<a href="https://github.com/owner/repo/commit/1111">1111</a>`, sdi.CommitRefIDLinkHTML(ctx, "1111"))
	assert.EqualValues(t, `<a href="https://github.com/owner/repo/compare/aaaa...bbbb">aaaa...bbbb</a>`, sdi.CompareRefIDLinkHTML(ctx))
	assert.EqualValues(t, `<a href="https://github.com/owner/repo">name</a>`, sdi.SubmoduleRepoLinkHTML(ctx))
}
