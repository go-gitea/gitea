// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const exampleDiff = `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`

const breakingDiff = `diff --git a/aaa.sql b/aaa.sql
index d8e4c92..19dc8ad 100644
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some coment 2
+-- some comment 3
 create or replace procedure test(p1 varchar2)
 is
 begin
---new comment
 dbms_output.put_line(p1);
+--some other comment
 end;
 /
`

var issue17875Diff = `diff --git a/Geschäftsordnung.md b/Geschäftsordnung.md
index d46c152..a7d2d55 100644
--- a/Geschäftsordnung.md
+++ b/Geschäftsordnung.md
@@ -1,5 +1,5 @@
 ---
-date: "23.01.2021"
+date: "30.11.2021"
 ...
 ` + `
 # Geschäftsordnung
@@ -16,4 +16,22 @@ Diese Geschäftsordnung regelt alle Prozesse des Vereins, solange diese nicht du
 ` + `
 ## § 3 Datenschutzverantwortlichkeit
 ` + `
-1. Der Verein bestellt eine datenschutzverantwortliche Person mit den Aufgaben nach Artikel 39 DSGVO.
\ No newline at end of file
+1. Der Verein bestellt eine datenschutzverantwortliche Person mit den Aufgaben nach Artikel 39 DSGVO.
+
+## §4 Umgang mit der SARS-Cov-2-Pandemie
+
+1. Der Vorstand hat die Befugnis, in Rücksprache mit den Vereinsmitgliedern, verschiedene Hygienemaßnahmen für Präsenzveranstaltungen zu beschließen.
+
+2. Die Einführung, Änderung und Abschaffung dieser Maßnahmen sind nur zum Zweck der Eindämmung der SARS-Cov-2-Pandemie zulässig.
+
+3. Die Einführung, Änderung und Abschaffung von Maßnahmen nach Abs. 2 bedarf einer wissenschaftlichen Grundlage.
+
+4. Die Maßnahmen nach Abs. 2 setzen sich aus den folgenden Bausteinen inklusive einer ihrer Ausprägungen zusammen.
+
+	1. Maskenpflicht: Keine; Maskenpflicht, außer am Platz, oder wo Abstände nicht eingehalten werden können; Maskenpflicht, wenn Abstände nicht eingehalten werden können;  Maskenpflicht
+
+	2. Geimpft-, Genesen- oder Testnachweis: Kein Nachweis notwendig; Nachweis, dass Person geimpft, genesen oder tagesaktuell getestet ist (3G); Nachweis, dass Person geimpft oder genesen ist (2G); Nachweis, dass Person geimpft bzw. genesen und tagesaktuell getestet ist (2G+)
+
+	3. Online-Veranstaltung: Keine, parallele Online-Veranstaltung, ausschließlich Online-Veranstaltung
+
+5. Bei Präsenzveranstungen gelten außerdem die Hygienevorschriften des Veranstaltungsorts. Bei Regelkollision greift die restriktivere Regel.
\ No newline at end of file`

func TestCutDiffAroundLineIssue17875(t *testing.T) {
	result, err := CutDiffAroundLine(strings.NewReader(issue17875Diff), 23, false, 3)
	assert.NoError(t, err)
	expected := `diff --git a/Geschäftsordnung.md b/Geschäftsordnung.md
--- a/Geschäftsordnung.md
+++ b/Geschäftsordnung.md
@@ -20,0 +21,3 @@
+## §4 Umgang mit der SARS-Cov-2-Pandemie
+
+1. Der Vorstand hat die Befugnis, in Rücksprache mit den Vereinsmitgliedern, verschiedene Hygienemaßnahmen für Präsenzveranstaltungen zu beschließen.`
	assert.Equal(t, expected, result)
}

func TestCutDiffAroundLine(t *testing.T) {
	result, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 4, false, 3)
	assert.NoError(t, err)
	resultByLine := strings.Split(result, "\n")
	assert.Len(t, resultByLine, 7)
	// Check if headers got transferred
	assert.Equal(t, "diff --git a/README.md b/README.md", resultByLine[0])
	assert.Equal(t, "--- a/README.md", resultByLine[1])
	assert.Equal(t, "+++ b/README.md", resultByLine[2])
	// Check if hunk header is calculated correctly
	assert.Equal(t, "@@ -2,2 +3,2 @@", resultByLine[3])
	// Check if line got transferred
	assert.Equal(t, "+ Build Status", resultByLine[4])

	// Must be same result as before since old line 3 == new line 5
	newResult, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	assert.NoError(t, err)
	assert.Equal(t, result, newResult, "Must be same result as before since old line 3 == new line 5")

	newResult, err = CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 300)
	assert.NoError(t, err)
	assert.Equal(t, exampleDiff, newResult)

	emptyResult, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 0)
	assert.NoError(t, err)
	assert.Empty(t, emptyResult)

	// Line is out of scope
	emptyResult, err = CutDiffAroundLine(strings.NewReader(exampleDiff), 434, false, 0)
	assert.NoError(t, err)
	assert.Empty(t, emptyResult)

	// Handle minus diffs properly
	minusDiff, err := CutDiffAroundLine(strings.NewReader(breakingDiff), 2, false, 4)
	assert.NoError(t, err)

	expected := `diff --git a/aaa.sql b/aaa.sql
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some coment 2`
	assert.Equal(t, expected, minusDiff)

	// Handle minus diffs properly
	minusDiff, err = CutDiffAroundLine(strings.NewReader(breakingDiff), 3, false, 4)
	assert.NoError(t, err)

	expected = `diff --git a/aaa.sql b/aaa.sql
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some coment 2
+-- some comment 3`

	assert.Equal(t, expected, minusDiff)
}

func BenchmarkCutDiffAroundLine(b *testing.B) {
	for b.Loop() {
		CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	}
}

func ExampleCutDiffAroundLine() {
	const diff = `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`
	result, _ := CutDiffAroundLine(strings.NewReader(diff), 4, false, 3)
	println(result)
}

func TestParseDiffHunkString(t *testing.T) {
	leftLine, leftHunk, rightLine, rightHunk := ParseDiffHunkString("@@ -19,3 +19,5 @@ AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER")
	assert.Equal(t, 19, leftLine)
	assert.Equal(t, 3, leftHunk)
	assert.Equal(t, 19, rightLine)
	assert.Equal(t, 5, rightHunk)
}
