// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaTemplate(t *testing.T) {
	giteaTemplate := []byte(`
# Header

# All .go files
**.go

# All text files in /text/
text/*.txt

# All files in modules folders
**/modules/*

# Exclude some files
!**/modules/*.tmp
`)

	gt := newGiteaTemplateFileMatcher("", giteaTemplate)
	assert.Len(t, gt.includeGlobs, 3)

	tt := []struct {
		Path    string
		Include bool
		Exclude bool
	}{
		{Path: "main.go", Include: true},
		{Path: "sub/sub/foo.go", Include: true},

		{Path: "a.txt"},
		{Path: "text/a.txt", Include: true},
		{Path: "sub/text/a.txt"},
		{Path: "text/a.json"},

		{Path: "a/b/c/modules/README.md", Include: true},
		{Path: "a/b/c/modules/README.md.tmp", Include: true, Exclude: true},
		{Path: "a/b/c/modules/d/README.md"},
	}

	for _, tc := range tt {
		assert.Equal(t, tc.Include, gt.matchRules(gt.includeGlobs, tc.Path), "should include path: %s", tc.Path)
		assert.Equal(t, tc.Exclude, gt.matchRules(gt.excludeGlobs, tc.Path), "should exclude path: %s", tc.Path)
	}
}

func TestFilePathSanitize(t *testing.T) {
	// path clean
	assert.Equal(t, "a", filePathSanitize("//a/"))
	assert.Equal(t, "_a", filePathSanitize(`\a`))
	assert.Equal(t, "__/a/__", filePathSanitize(".. /a/ .."))
	assert.Equal(t, "__/a/_git/b_", filePathSanitize("./../a/.git/ b: "))

	// Windows reserved names
	assert.Equal(t, "_", filePathSanitize("CoN"))
	assert.Equal(t, "_", filePathSanitize("LpT1"))
	assert.Equal(t, "_", filePathSanitize("CoM1"))
	assert.Equal(t, "test_CON", filePathSanitize("test_CON"))
	assert.Equal(t, "test CON", filePathSanitize("test CON "))

	// special chars
	assert.Equal(t, "_", filePathSanitize("\u0000"))
	assert.Equal(t, ".", filePathSanitize(""))
	assert.Equal(t, ".", filePathSanitize("."))
	assert.Equal(t, ".", filePathSanitize("/"))
}

func TestProcessGiteaTemplateFileGenerate(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "gitea-template-test")

	assertFileContent := func(path, expected string) {
		data, err := os.ReadFile(filepath.Join(tmpDir, path))
		if expected == "" {
			assert.ErrorIs(t, err, os.ErrNotExist)
			return
		}
		require.NoError(t, err)
		assert.Equal(t, expected, string(data), "file content mismatch for %s", path)
	}

	assertSymLink := func(path, expected string) {
		link, err := os.Readlink(filepath.Join(tmpDir, path))
		if expected == "" {
			assert.ErrorIs(t, err, os.ErrNotExist)
			return
		}
		require.NoError(t, err)
		assert.Equal(t, expected, link, "symlink target mismatch for %s", path)
	}

	require.NoError(t, os.MkdirAll(tmpDir+"/.git", 0o755))
	require.NoError(t, os.WriteFile(tmpDir+"/.git/config", []byte("git-config-dummy"), 0o644))
	require.NoError(t, os.MkdirAll(tmpDir+"/.gitea", 0o755))
	require.NoError(t, os.WriteFile(tmpDir+"/.gitea/template", []byte("*\ninclude/**"), 0o644))
	require.NoError(t, os.MkdirAll(tmpDir+"/sub", 0o755))
	require.NoError(t, os.MkdirAll(tmpDir+"/include/foo/bar", 0o755))

	require.NoError(t, os.WriteFile(tmpDir+"/sub/link-target", []byte("link target content from ${TEMPLATE_NAME}"), 0o644))
	require.NoError(t, os.WriteFile(tmpDir+"/include/foo/bar/test.txt", []byte("include subdir ${TEMPLATE_NAME}"), 0o644))

	// case-1
	{
		require.NoError(t, os.WriteFile(tmpDir+"/normal", []byte("normal content"), 0o644))
		require.NoError(t, os.WriteFile(tmpDir+"/template", []byte("template from ${TEMPLATE_NAME}"), 0o644))
	}

	// case-2
	{
		require.NoError(t, os.Symlink(tmpDir+"/sub/link-target", tmpDir+"/link"))
	}

	// case-3
	{
		require.NoError(t, os.WriteFile(tmpDir+"/subst-${REPO_NAME}", []byte("dummy subst repo name"), 0o644))
	}

	// case-4
	assertSubstTemplateName := func(normalContent, toLinkContent, fromLinkContent string) {
		assertFileContent("subst-${TEMPLATE_NAME}-normal", normalContent)
		assertFileContent("subst-${TEMPLATE_NAME}-to-link", toLinkContent)
		assertFileContent("subst-${TEMPLATE_NAME}-from-link", fromLinkContent)
	}

	// case-5
	{
		require.NoError(t, os.MkdirAll(tmpDir+"/real-dir", 0o755))
		require.NoError(t, os.WriteFile(tmpDir+"/real-dir/real-file", []byte("origin content"), 0o644))
		require.NoError(t, os.MkdirAll(tmpDir+"/include/subst-${TEMPLATE_NAME}-link-dir", 0o755))
		require.NoError(t, os.WriteFile(tmpDir+"/include/subst-${TEMPLATE_NAME}-link-dir/real-file", []byte("template content"), 0o644))
		require.NoError(t, os.Symlink(tmpDir+"/real-dir", tmpDir+"/include/subst-TemplateRepoName-link-dir"))
	}

	{
		// will succeed
		require.NoError(t, os.WriteFile(tmpDir+"/subst-${TEMPLATE_NAME}-normal", []byte("dummy subst template name normal"), 0o644))
		// will be skipped if the path subst result is a link
		require.NoError(t, os.WriteFile(tmpDir+"/subst-${TEMPLATE_NAME}-to-link", []byte("dummy subst template name to link"), 0o644))
		require.NoError(t, os.Symlink(tmpDir+"/sub/link-target", tmpDir+"/subst-TemplateRepoName-to-link"))
		// will be skipped since the source is a symlink
		require.NoError(t, os.Symlink(tmpDir+"/sub/link-target", tmpDir+"/subst-${TEMPLATE_NAME}-from-link"))
		// pre-check
		assertSubstTemplateName("dummy subst template name normal", "dummy subst template name to link", "link target content from ${TEMPLATE_NAME}")
	}

	// process the template files
	{
		templateRepo := &repo_model.Repository{Name: "TemplateRepoName"}
		generatedRepo := &repo_model.Repository{Name: "/../.gIt/name"}
		assertFileContent(".git/config", "git-config-dummy")
		fileMatcher, _ := readGiteaTemplateFile(tmpDir)
		skippedFiles, err := processGiteaTemplateFile(t.Context(), tmpDir, templateRepo, generatedRepo, fileMatcher)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"include/subst-${TEMPLATE_NAME}-link-dir/real-file",
			"include/subst-TemplateRepoName-link-dir",
			"link",
			"subst-${TEMPLATE_NAME}-from-link",
			"subst-${TEMPLATE_NAME}-to-link",
			"subst-TemplateRepoName-to-link",
		}, skippedFiles)
		assertFileContent(".git/config", "")
		assertFileContent(".gitea/template", "")
		assertFileContent("include/foo/bar/test.txt", "include subdir TemplateRepoName")
	}

	// the lin target should never be modified, and since it is in a subdirectory, it is not affected by the template either
	assertFileContent("sub/link-target", "link target content from ${TEMPLATE_NAME}")

	// case-1
	{
		assertFileContent("no-such", "")
		assertFileContent("normal", "normal content")
		assertFileContent("template", "template from TemplateRepoName")
	}

	// case-2
	{
		// symlink with templates should be preserved (not read or write)
		assertSymLink("link", tmpDir+"/sub/link-target")
	}

	// case-3
	{
		assertFileContent("subst-${REPO_NAME}", "")
		assertFileContent("subst-/__/_gIt/name", "dummy subst repo name")
	}

	// case-4
	{
		// the paths with templates should have been removed, subst to a regular file, succeed, the link is preserved
		assertSubstTemplateName("", "", "link target content from ${TEMPLATE_NAME}")
		assertFileContent("subst-TemplateRepoName-normal", "dummy subst template name normal")
		// subst to a link, skip, and the target is unchanged
		assertSymLink("subst-TemplateRepoName-to-link", tmpDir+"/sub/link-target")
		// subst from a link, skip, and the target is unchanged
		assertSymLink("subst-${TEMPLATE_NAME}-from-link", tmpDir+"/sub/link-target")
	}

	// case-5
	{
		assertFileContent("real-dir/real-file", "origin content")
	}
}

func TestProcessGiteaTemplateFileRead(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(tmpDir+"/.gitea", 0o755)
	templateFilePath := tmpDir + "/.gitea/template"
	_ = os.Remove(templateFilePath)
	_, err := os.Lstat(templateFilePath)
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = readGiteaTemplateFile(tmpDir) // no template file
	require.ErrorIs(t, err, fs.ErrNotExist)

	_ = os.WriteFile(templateFilePath+".target", []byte("test-data-target"), 0o644)
	_ = os.Symlink(templateFilePath+".target", templateFilePath)
	content, _ := os.ReadFile(templateFilePath)
	require.Equal(t, "test-data-target", string(content))
	_, err = readGiteaTemplateFile(tmpDir) // symlinked template file
	require.ErrorIs(t, err, fs.ErrNotExist)

	_ = os.Remove(templateFilePath)
	_ = os.WriteFile(templateFilePath, []byte("test-data-regular"), 0o644)
	content, _ = os.ReadFile(templateFilePath)
	require.Equal(t, "test-data-regular", string(content))
	fm, err := readGiteaTemplateFile(tmpDir) // regular template file
	require.NoError(t, err)
	assert.Len(t, fm.includeGlobs, 1)
}

func TestProcessGiteaTemplateFileExclusion(t *testing.T) {
	t.Run("include and exclude patterns", func(t *testing.T) {
		tmpDir := t.TempDir()

		assertFileExists := func(path string, shouldExist bool) {
			_, err := os.Stat(filepath.Join(tmpDir, path))
			if shouldExist {
				require.NoError(t, err, "file should exist: %s", path)
			} else {
				require.True(t, os.IsNotExist(err), "file should not exist: %s", path)
			}
		}

		assertFileContent := func(path, expected string) {
			data, err := os.ReadFile(filepath.Join(tmpDir, path))
			if expected == "" {
				require.True(t, os.IsNotExist(err), "file should not exist: %s", path)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, expected, string(data))
		}

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".gitea"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitea", "template"),
			[]byte("*.go\n*.md\n!vendor\n!*.log"), 0o644))

		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"),
			[]byte("package ${REPO_NAME}"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.md"),
			[]byte("# ${REPO_NAME}"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "vendor"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vendor", "dep.go"),
			[]byte("vendor code"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "error.log"),
			[]byte("log content"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build.log"),
			[]byte("build ${REPO_NAME}"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Makefile"),
			[]byte("build: main.go"), 0o644))
		require.NoError(t, os.Symlink(filepath.Join(tmpDir, "Makefile"),
			filepath.Join(tmpDir, "link-to-makefile")))

		templateRepo := &repo_model.Repository{Name: "MyTemplate"}
		generatedRepo := &repo_model.Repository{Name: "MyRepo"}

		fm, err := readGiteaTemplateFile(tmpDir)
		require.NoError(t, err)
		require.Len(t, fm.includeGlobs, 2)
		require.Len(t, fm.excludeGlobs, 2)

		_, err = processGiteaTemplateFile(t.Context(), tmpDir, templateRepo, generatedRepo, fm)
		require.NoError(t, err)

		assertFileContent("main.go", "package MyRepo")
		assertFileContent("readme.md", "# MyRepo")
		assertFileExists("vendor", false)
		assertFileExists("vendor/dep.go", false)
		assertFileExists("error.log", false)
		assertFileExists("build.log", false)
		assertFileContent("Makefile", "build: main.go")
		assertFileExists("link-to-makefile", true)
	})

	t.Run("only exclude, no include rules", func(t *testing.T) {
		tmpDir := t.TempDir()

		assertFileExists := func(path string, shouldExist bool) {
			_, err := os.Stat(filepath.Join(tmpDir, path))
			if shouldExist {
				require.NoError(t, err, "file should exist: %s", path)
			} else {
				require.True(t, os.IsNotExist(err), "file should not exist: %s", path)
			}
		}

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".gitea"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitea", "template"),
			[]byte("!vendor\n!*.tmp"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "vendor"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vendor", "pkg.go"), []byte("vendor"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "temp.tmp"), []byte("tmp"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("keep"), 0o644))

		templateRepo := &repo_model.Repository{Name: "T"}
		generatedRepo := &repo_model.Repository{Name: "R"}

		fm, err := readGiteaTemplateFile(tmpDir)
		require.NoError(t, err)

		_, err = processGiteaTemplateFile(t.Context(), tmpDir, templateRepo, generatedRepo, fm)
		require.NoError(t, err)

		assertFileExists("vendor", false)
		assertFileExists("vendor/pkg.go", false)
		assertFileExists("temp.tmp", false)
		assertFileExists("main.go", true)
	})

	t.Run("symlink-to-dir and gitea contents", func(t *testing.T) {
		tmpDir := t.TempDir()

		assertFileExists := func(path string, shouldExist bool) {
			_, err := os.Stat(filepath.Join(tmpDir, path))
			if shouldExist {
				require.NoError(t, err, "file should exist: %s", path)
			} else {
				require.True(t, os.IsNotExist(err), "file should not exist: %s", path)
			}
		}

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "realdir"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "realdir", "keep.txt"), []byte("keep"), 0o644))
		require.NoError(t, os.Symlink(filepath.Join(tmpDir, "realdir"), filepath.Join(tmpDir, "link-to-dir")))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".gitea"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitea", "template"),
			[]byte("!link-to-dir/*\n!.gitea/delete-me.txt"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitea", "delete-me.txt"), []byte("x"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitea", "keep-me.txt"), []byte("y"), 0o644))

		templateRepo := &repo_model.Repository{Name: "T"}
		generatedRepo := &repo_model.Repository{Name: "R"}

		fm, err := readGiteaTemplateFile(tmpDir)
		require.NoError(t, err)

		_, err = processGiteaTemplateFile(t.Context(), tmpDir, templateRepo, generatedRepo, fm)
		require.NoError(t, err)

		// Symlink to directory survives (not followed, not deleted)
		assertFileExists("link-to-dir", true)
		assertFileExists("realdir/keep.txt", true)

		// Matched file inside .gitea/ is deleted
		assertFileExists(".gitea/delete-me.txt", false)
		// Unmatched file inside .gitea/ survives
		assertFileExists(".gitea/keep-me.txt", true)
		// .gitea/ template has already been cleaned up
		assertFileExists(".gitea/template", false)
	})
}

func TestTransformers(t *testing.T) {
	cases := []struct {
		name     string
		expected string
	}{
		{"SNAKE", "abc_def_xyz"},
		{"KEBAB", "abc-def-xyz"},
		{"CAMEL", "abcDefXyz"},
		{"PASCAL", "AbcDefXyz"},
		{"LOWER", "abc_def-xyz"},
		{"UPPER", "ABC_DEF-XYZ"},
		{"TITLE", "Abc_def-Xyz"},
	}

	input := "Abc_Def-XYZ"
	assert.Len(t, globalVars().defaultTransformers, len(cases))
	for i, c := range cases {
		tf := globalVars().defaultTransformers[i]
		require.Equal(t, c.name, tf.Name)
		assert.Equal(t, c.expected, tf.Transform(input), "case %s", c.name)
	}
}
