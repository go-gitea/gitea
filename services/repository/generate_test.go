// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

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
[exclude]
**/modules/*.tmp
`)

	gt := newGiteaTemplateFileMatcher("", giteaTemplate)
	assert.Len(t, gt.globsExpand, 3)

	tt := []struct {
		Path    string
		Expand  bool
		Exclude bool
	}{
		{Path: "main.go", Expand: true},
		{Path: "sub/sub/foo.go", Expand: true},

		{Path: "a.txt"},
		{Path: "text/a.txt", Expand: true},
		{Path: "sub/text/a.txt"},
		{Path: "text/a.json"},

		{Path: "a/b/c/modules/README.md", Expand: true},
		{Path: "a/b/c/modules/README.md.tmp", Expand: true, Exclude: true},
		{Path: "a/b/c/modules/d/README.md"},
	}

	for _, tc := range tt {
		assert.Equal(t, tc.Expand, gt.matchRules(gt.globsExpand, tc.Path), "should expand: %s", tc.Path)
		assert.Equal(t, tc.Exclude, gt.matchRules(gt.globsExclude, tc.Path), "should exclude: %s", tc.Path)
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
	fh := test.NewFileHelper(tmpDir)

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
		// empty as non-existing
		fh.AssertFileExists(t, "subst-${TEMPLATE_NAME}-normal", util.Iif(normalContent == "", nil, new(normalContent)))
		fh.AssertFileExists(t, "subst-${TEMPLATE_NAME}-to-link", util.Iif(toLinkContent == "", nil, new(toLinkContent)))
		fh.AssertFileExists(t, "subst-${TEMPLATE_NAME}-from-link", util.Iif(fromLinkContent == "", nil, new(fromLinkContent)))
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
		fh.AssertFileContent(t, ".git/config", "git-config-dummy")
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
		fh.AssertExists(t, ".git/config", false)
		fh.AssertExists(t, ".gitea/template", false)
		fh.AssertFileContent(t, "include/foo/bar/test.txt", "include subdir TemplateRepoName")
	}

	// the lin target should never be modified, and since it is in a subdirectory, it is not affected by the template either
	fh.AssertFileContent(t, "sub/link-target", "link target content from ${TEMPLATE_NAME}")

	// case-1
	{
		fh.AssertExists(t, "no-such", false)
		fh.AssertFileContent(t, "normal", "normal content")
		fh.AssertFileContent(t, "template", "template from TemplateRepoName")
	}

	// case-2
	{
		// symlink with templates should be preserved (not read or write)
		fh.AssertSymLink(t, "link", "sub/link-target")
	}

	// case-3
	{
		fh.AssertExists(t, "subst-${REPO_NAME}", false)
		fh.AssertFileContent(t, "subst-/__/_gIt/name", "dummy subst repo name")
	}

	// case-4
	{
		// the paths with templates should have been removed, subst to a regular file, succeed, the link is preserved
		assertSubstTemplateName("", "", "link target content from ${TEMPLATE_NAME}")
		fh.AssertFileContent(t, "subst-TemplateRepoName-normal", "dummy subst template name normal")
		// subst to a link, skip, and the target is unchanged
		fh.AssertSymLink(t, "subst-TemplateRepoName-to-link", "sub/link-target")
		// subst from a link, skip, and the target is unchanged
		fh.AssertSymLink(t, "subst-${TEMPLATE_NAME}-from-link", "sub/link-target")
	}

	// case-5
	{
		fh.AssertFileContent(t, "real-dir/real-file", "origin content")
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
	assert.Len(t, fm.globsExpand, 1)
}

func TestProcessGiteaTemplateFileExclusion(t *testing.T) {
	tmpDir := t.TempDir()
	fh := test.NewFileHelper(tmpDir)

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".gitea"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitea", "template"), []byte(`
[expand]
*.md
*.txt

[exclude]
*.log
`), 0o644))
	fh.WriteFile(t, "test.md", "from ${TEMPLATE_NAME}")
	fh.WriteFile(t, "test.go", "package test")
	fh.WriteFile(t, "test.log", "log-content")
	fh.MkdirAll(t, "subdir")
	fh.WriteFile(t, "subdir/foo", "bar")
	fh.Symlink(t, "test.go", "symlink.go")
	fh.Symlink(t, "test.log", "symlink.log")
	fh.Symlink(t, "subdir", "symlink-dir.log")

	templateRepo := &repo_model.Repository{Name: "MyTemplate"}
	generatedRepo := &repo_model.Repository{Name: "MyRepo"}

	fm, err := readGiteaTemplateFile(tmpDir)
	require.NoError(t, err)
	require.Len(t, fm.globsExpand, 2)
	require.Len(t, fm.globsExclude, 1)

	_, err = processGiteaTemplateFile(t.Context(), tmpDir, templateRepo, generatedRepo, fm)
	require.NoError(t, err)

	fh.AssertFileContent(t, "test.md", "from MyTemplate")
	fh.AssertFileContent(t, "test.go", "package test")
	fh.AssertExists(t, "test.log", false)
	fh.AssertFileContent(t, "subdir/foo", "bar")
	fh.AssertSymLink(t, "symlink.go", "test.go")
	fh.AssertExists(t, "symlink.log", false)
	fh.AssertExists(t, "symlink-dir.log", false)
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
