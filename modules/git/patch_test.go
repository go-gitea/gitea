// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyDiff(t *testing.T) {
	assert.Equal(t, "", GetDiffAsUnifiedPatch("", ""))
}

func TestNoDifference(t *testing.T) {
	assert.Equal(t, "", GetDiffAsUnifiedPatch("", ""))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("a", "a"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("a\n", "a\n"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("a\r", "a\r"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("a\t", "a\t"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("@@", "@@"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf", "a\nb\nc\nd\ne\nf"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf", "a\nb\nc\nd\ne\nf"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("\n\n \n\n\n\n", "\n\n \n\n\n\n"))
	assert.Equal(t, "", GetDiffAsUnifiedPatch("@@ -1,6 +1,5 @@\n a\n-b\n c\n d\n e\n f\n", "@@ -1,6 +1,5 @@\n a\n-b\n c\n d\n e\n f\n"))
}
func TestNoDifferenceGitDiff(t *testing.T) {
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "", ""))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "a", "a"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "a\n", "a\n"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "a\r", "a\r"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "a\t", "a\t"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "@@", "@@"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "a\nb\nc\nd\ne\nf", "a\nb\nc\nd\ne\nf"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "a\nb\nc\nd\ne\nf", "a\nb\nc\nd\ne\nf"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "\n\n \n\n\n\n", "\n\n \n\n\n\n"))
	assert.Equal(t, "", GetUnifiedGitDiff("dir/filename.txt", "@@ -1,6 +1,5 @@\n a\n-b\n c\n d\n e\n f\n", "@@ -1,6 +1,5 @@\n a\n-b\n c\n d\n e\n f\n"))
}

func TestWindowsLineEnding(t *testing.T) {
	assert.Equal(t, "@@ -1,4 +0,0 @@\n-a\r\n-b\n-c\rd\n-e\r\n", GetDiffAsUnifiedPatch("a\r\nb\nc\rd\ne\r\n", ""))
}

func TestNoNewLineEOL(t *testing.T) {
	assert.Equal(t, "@@ -1,4 +0,0 @@\n-a\r\n-b\n-c\rd\n-e\n\\ No newline at end of file\n", GetDiffAsUnifiedPatch("a\r\nb\nc\rd\ne", ""))
	// FIXME: verfiy compare these to GNU diff output:
	assert.Equal(t, "@@ -0,0 +1 @@\n+\n", GetDiffAsUnifiedPatch("", "\n"))
	assert.Equal(t, "@@ -1 +0,0 @@\n-\n", GetDiffAsUnifiedPatch("\n", ""))

	assert.Equal(t, "@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+a\n", GetDiffAsUnifiedPatch("a", "a\n"))
	assert.Equal(t, "@@ -1 +1 @@\n-a\n+a\n\\ No newline at end of file\n", GetDiffAsUnifiedPatch("a\n", "a"))
}

func TestSimpleOneChunkAtTheBegginingPatch(t *testing.T) {
	assert.Equal(t, "@@ -1,5 +1,5 @@\n-a\n+b\n b\n c\n d\n e\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "b\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -1,6 +1,6 @@\n-a\n b\n+a\n c\n d\n e\n f\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "b\na\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -1,8 +1,9 @@\n+b\n a\n b\n c\n d\n e\n f\n g\n h\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "b\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -1,6 +1,6 @@\n a\n-b\n+\n c\n d\n e\n f\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "a\n\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -1,6 +1,5 @@\n a\n-b\n c\n d\n e\n f\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "a\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
}

func TestSimpleOneChunkAtTheMiddlePatch(t *testing.T) {
	assert.Equal(t, "@@ -5,9 +5,9 @@\n e\n f\n g\n h\n-i \n+i\n j\n k\n l\n m\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni \nj\nk\nl\nm\nn\no\np\nq\n", "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -5,9 +5,10 @@\n e\n f\n g\n h\n-i \n+i\n+j\n k\n l\n m\n n\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni \nk\nl\nm\nn\no\np\nq\n", "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -5,10 +5,9 @@\n e\n f\n g\n h\n-i\n-j\n+i \n k\n l\n m\n n\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "a\nb\nc\nd\ne\nf\ng\nh\ni \nk\nl\nm\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -4,14 +4,8 @@\n d\n e\n f\n g\n-h\n-i\n-j\n-k\n-l\n-m\n n\n o\n p\n q\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\n", "a\nb\nc\nd\ne\nf\ng\nn\no\np\nq\n"))
	assert.Equal(t, "@@ -5,14 +5,8 @@\n e\n f\n g\n h\n-i\n-j\n-k\n-l\n-m\n-n\n o\n p\n q\n r\n@@ -15,10 +15,8 @@\n u\n v\n w\n x\n-y\n-z\n A\n B\n C\n D\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\nr\ns\nt\nu\nv\nw\nx\ny\nz\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT", "a\nb\nc\nd\ne\nf\ng\nh\no\np\nq\nr\ns\nt\nu\nv\nw\nx\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT"))
	assert.Equal(t, "@@ -5,15 +5,10 @@\n e\n f\n g\n h\n-i\n-j\n-k\n-l\n-m\n-n\n o\n+\n p\n q\n r\n s\n@@ -16,10 +16,8 @@\n u\n v\n w\n x\n-y\n-z\n A\n B\n C\n D\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\nr\ns\nt\nu\nv\nw\nx\ny\nz\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT", "a\nb\nc\nd\ne\nf\ng\nh\no\n\np\nq\nr\ns\nt\nu\nv\nw\nx\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT"))
	assert.Equal(t, "@@ -5,14 +5,8 @@\n e\n f\n g\n h\n-i\n-j\n-k\n-l\n-m\n-n\n o\n p\n q\n r\n@@ -15,10 +15,8 @@\n u\n v\n w\n x\n-y\n-z\n A\n B\n C\n D\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\nr\ns\nt\nu\nv\nw\nx\ny\nz\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT", "a\nb\nc\nd\ne\nf\ng\nh\no\np\nq\nr\ns\nt\nu\nv\nw\nx\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT"))
	assert.Equal(t, "@@ -5,14 +5,9 @@\n e\n f\n g\n h\n-i\n-j\n-k\n-l\n-m\n-n\n+\n o\n p\n q\n r\n@@ -16,10 +16,8 @@\n u\n v\n w\n x\n-y\n-z\n A\n B\n C\n D\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\nr\ns\nt\nu\nv\nw\nx\ny\nz\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT", "a\nb\nc\nd\ne\nf\ng\nh\n\no\np\nq\nr\ns\nt\nu\nv\nw\nx\nA\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT"))
}

func TestAlternatingEditsAndEquals(t *testing.T) {
	// neither diff nor git diff minimises the number of edits.
	assert.Equal(t, "@@ -1,8 +1,8 @@\n a\n-b\n+4\n c\n-d\n+5\n e\n-f\n+6\n g\n h\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\n", "a\n4\nc\n5\ne\n6\ng\nh\n"))
	assert.Equal(t, "@@ -1,8 +1,8 @@\n a\n-b\n+4\n c\n-d\n+5\n e\n-f\n+6\n g\n-h\n+h\n\\ No newline at end of file\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh\n", "a\n4\nc\n5\ne\n6\ng\nh"))
	assert.Equal(t, "@@ -1,8 +1,8 @@\n a\n-b\n+4\n c\n-d\n+5\n e\n-f\n+6\n g\n h\n\\ No newline at end of file\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh", "a\n4\nc\n5\ne\n6\ng\nh"))
	assert.Equal(t, "@@ -1,8 +1,8 @@\n a\n-b\n+4\n c\n-d\n+5\n e\n-f\n+6\n g\n-h\n\\ No newline at end of file\n+h\n", GetDiffAsUnifiedPatch("a\nb\nc\nd\ne\nf\ng\nh", "a\n4\nc\n5\ne\n6\ng\nh\n"))
}

func TestUniqueContextGeneration(t *testing.T) {
	const FILLER = "a\nb\nc\nd\ne\n"

	textA := FILLER + FILLER + FILLER + "abcd444\n" + FILLER + FILLER + FILLER + FILLER + FILLER + FILLER + "UNIQUE\n" + FILLER + FILLER + FILLER + "abcd444\n" + FILLER + FILLER + FILLER + FILLER + FILLER + FILLER
	textB := FILLER + FILLER + FILLER + "abcd444\n" + FILLER + FILLER + FILLER + FILLER + FILLER + FILLER + "UNIQUE\n" + FILLER + FILLER + FILLER + "changed\n" + FILLER + FILLER + FILLER + FILLER + FILLER + FILLER

	assert.Equal(t, "@@ -47,33 +47,33 @@\n UNIQUE\n a\n b\n c\n d\n e\n a\n b\n c\n d\n e\n a\n b\n c\n d\n e\n-abcd444\n+changed\n a\n b\n c\n d\n e\n a\n b\n c\n d\n e\n a\n b\n c\n d\n e\n a\n", GetDiffAsUnifiedPatch(textA, textB))
}

func TestGitDiffHeader(t *testing.T) {
	assert.Equal(t, "diff --git a/dir/filename.txt b/dir/filename.txt\n--- a/dir/filename.txt\n+++ b/dir/filename.txt\n@@ -1,2 +0,0 @@\n-a\n-b\n\\ No newline at end of file\n", GetUnifiedGitDiff("dir/filename.txt", "a\nb", ""))
}

func TestPatchToGitDiffSimple(t *testing.T) {
	assert.Equal(t, GetUnifiedGitDiff("dir/filename.txt", "a\nb", ""), PatchToGitDiff(GetDiffAsUnifiedPatchWithFilenames(false, "a/dir/filename.txt", "b/dir/filename.txt", "a\nb", "")))
}

func GetGitDiff(touples ...string) string {
	if len(touples)%4 != 0 {
		panic("invalid test data")
	}

	result := ""

	for i := 0; i < len(touples); i += 4 {
		result += "\n\n" + GetDiffAsUnifiedPatchWithFilenames(true, touples[i], touples[i+1], touples[i+2], touples[i+3]) + "\n"
	}

	return result
}

func GetUnifiedPatch(touples ...string) string {
	if len(touples)%4 != 0 {
		panic("invalid test data")
	}

	result := ""

	for i := 0; i < len(touples); i += 4 {
		result += GetDiffAsUnifiedPatchWithFilenames(false, touples[i], touples[i+1], touples[i+2], touples[i+3]) + "\n\n\n\n\n"
	}

	return result
}

func removeCompletelyEmptyLines(s string) string {
	lines := strings.Split(s, "\n")

	var text bytes.Buffer

	for _, line := range lines {
		if line != "" {
			text.WriteString(line)
			text.WriteString("\n")
		}
	}

	return text.String()
}

func TestPatchToGitDiffMultiFile(t *testing.T) {
	file1 := "dir/file1.txt"
	file1a := "a\nb"
	file1b := ""

	file2 := "dir/file2.txt"
	file2a := "a\nb"
	file2b := "a\nb"

	file3 := "dir/file3.txt"
	file3a := ""
	file3b := "b\na"

	file4 := "dir/file4.txt"
	file4a := ""
	file4b := ""

	file5 := "dir/file5.txt"
	file5a := "a\nb\nc\nd\ne\nf\ng\nh\n"
	file5b := "a\n4\nc\n5\ne\n6\ng\nh\n"

	file6 := "dir/file6.txt"
	file6a := ""
	file6b := "\r"
	assert.Equal(t, removeCompletelyEmptyLines(GetGitDiff(
		"a/"+file1, "b/"+file1, file1a, file1b,
		"a/"+file2, "b/"+file2, file2a, file2b,
		"a/"+file3, "b/"+file3, file3a, file3b,
		"a/"+file4, "b/"+file4, file4a, file4b,
		"a/"+file5, "b/"+file5, file5a, file5b,
		"a/"+file6, "b/"+file6, file6a, file6b)),
		removeCompletelyEmptyLines(PatchToGitDiff(GetUnifiedPatch(
			"a/"+file1, "b/"+file1, file1a, file1b,
			"a/"+file2, "b/"+file2, file2a, file2b,
			"a/"+file3, "b/"+file3, file3a, file3b,
			"a/"+file4, "b/"+file4, file4a, file4b,
			"a/"+file5, "b/"+file5, file5a, file5b,
			"a/"+file6, "b/"+file6, file6a, file6b))))
}
