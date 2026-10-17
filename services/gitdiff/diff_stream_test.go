// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitDiffStream(t *testing.T) {
	input := `diff --git "\\a/src/a.go" "\\b/src/a.go"
index 1111111..2222222 100644
--- "\\a/src/a.go"
+++ "\\b/src/a.go"
@@ -1,2 +1,2 @@
 package a
-var x = 1
+var x = 2
diff --git "\\a/old name.txt" "\\b/new name.txt"
similarity index 100%
rename from old name.txt
rename to new name.txt
diff --git "\\a/img.png" "\\b/img.png"
new file mode 100644
index 0000000..3333333
Binary files /dev/null and "\\b/img.png" differ
diff --git "\\a/run.sh" "\\b/run.sh"
old mode 100644
new mode 100755
diff --git "\\a/sub" "\\b/sub"
index 4444444..5555555 160000
--- "\\a/sub"
+++ "\\b/sub"
@@ -1 +1 @@
-Subproject commit 4444444444444444444444444444444444444444
+Subproject commit 5555555555555555555555555555555555555555
diff --git "\\a/big.txt" "\\b/big.txt"
deleted file mode 100644
--- "\\a/big.txt"
+++ /dev/null
@@ -1,3 +0,0 @@
-aaaaaaaaaa
-bbbbbbbbbb
-cccccccccc
`
	split := func(bufferSize int, maxFileSize, maxSize int64) (files []*DiffStreamFile, truncated bool) {
		truncated, err := splitDiffStream(bufio.NewReaderSize(strings.NewReader(input), bufferSize), maxFileSize, maxSize, func(file *DiffStreamFile, hunks *bytes.Buffer) error {
			file.Hunks = hunks.String()
			files = append(files, file)
			return nil
		})
		require.NoError(t, err)
		return files, truncated
	}
	files, truncated := split(16, 50, -1) // the smallest buffer splits the lines into chunks
	assert.False(t, truncated)
	require.Len(t, files, 6)

	assert.Equal(t, "src/a.go", files[0].Name)
	assert.Equal(t, "change", files[0].Type)
	assert.Empty(t, files[0].Mode)
	assert.Equal(t, "@@ -1,2 +1,2 @@\n package a\n-var x = 1\n+var x = 2\n", files[0].Hunks)
	assert.NotEmpty(t, files[0].NameHash)

	assert.Equal(t, "new name.txt", files[1].Name)
	assert.Equal(t, "old name.txt", files[1].OldName)
	assert.Equal(t, "rename", files[1].Type)
	assert.Zero(t, files[1].hunksSize)

	assert.Equal(t, "add", files[2].Type)
	assert.True(t, files[2].IsBinary)

	assert.Equal(t, "100644", files[3].OldMode)
	assert.Equal(t, "100755", files[3].Mode)

	require.NotNil(t, files[4].Submodule)
	assert.Equal(t, strings.Repeat("4", 40), files[4].Submodule.OldCommit)
	assert.Equal(t, strings.Repeat("5", 40), files[4].Submodule.NewCommit)

	assert.Equal(t, "delete", files[5].Type)
	assert.Equal(t, "big.txt", files[5].Name)
	assert.True(t, files[5].TooLarge)
	assert.Empty(t, files[5].Hunks)
	assert.Equal(t, 52, files[5].hunksSize)

	// a file beyond maxSize is too large and doesn't count, a later file beyond the rest of maxSize truncates the diff
	files, truncated = split(4096, -1, 110)
	assert.False(t, truncated)
	require.Len(t, files, 6)
	assert.True(t, files[4].TooLarge)
	assert.False(t, files[5].TooLarge)
	files, truncated = split(4096, -1, 60)
	assert.True(t, truncated)
	assert.Len(t, files, 5)
}

func TestDecodeHunksToUTF8(t *testing.T) {
	assert.Equal(t, "@@ -1 +1 @@\n-ä\n+ö\n", decodeHunksToUTF8([]byte("@@ -1 +1 @@\n-ä\n+ö\n")))
	// the old side is ISO-8859-1, the new side is UTF-8
	latin1 := []byte("@@ -1 +1 @@\n-caf\xe9 au lait, cr\xe8me br\xfbl\xe9e\n+café au lait, crème brûlée\n")
	assert.Equal(t, "@@ -1 +1 @@\n-café au lait, crème brûlée\n+café au lait, crème brûlée\n", decodeHunksToUTF8(latin1))
}

func TestSplitDiffStreamSubmodules(t *testing.T) {
	type testcase struct {
		name       string
		gitdiff    string
		submodules map[int]DiffStreamSubmodule
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
			submodules: map[int]DiffStreamSubmodule{
				1: {NewCommit: "68972a994719ae5c74e28d8fa82fa27c23399bc8"},
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
			submodules: map[int]DiffStreamSubmodule{
				0: {
					OldCommit: "68972a994719ae5c74e28d8fa82fa27c23399bc8",
					NewCommit: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
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
			submodules: map[int]DiffStreamSubmodule{
				1: {
					OldCommit: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
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
			submodules: map[int]DiffStreamSubmodule{
				1: {
					OldCommit: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
				},
				2: {
					NewCommit: "8eefa1f6dedf2488db2c9e12c916e8e51f673160",
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
			submodules: map[int]DiffStreamSubmodule{
				1: {
					OldCommit: "c8ffe777cf9c5bb47a38e3e0b3a3b5de6cd8813d",
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
			submodules: map[int]DiffStreamSubmodule{
				2: {
					NewCommit: "68972a994719ae5c74e28d8fa82fa27c23399bc8",
				},
			},
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			var files []*DiffStreamFile
			_, err := splitDiffStream(bufio.NewReader(strings.NewReader(testcase.gitdiff)), -1, -1, func(file *DiffStreamFile, _ *bytes.Buffer) error {
				files = append(files, file)
				return nil
			})
			require.NoError(t, err)
			for i, file := range files {
				if expected, ok := testcase.submodules[i]; ok {
					assert.Equal(t, expected, *file.Submodule)
				} else {
					assert.Nil(t, file.Submodule)
				}
			}
		})
	}
}
