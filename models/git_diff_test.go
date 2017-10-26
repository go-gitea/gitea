package models

import (
	"html/template"
	"testing"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

func assertEqual(t *testing.T, s1 string, s2 template.HTML) {
	if s1 != string(s2) {
		t.Errorf("%s should be equal %s", s2, s1)
	}
}

func assertLineEqual(t *testing.T, d1 *DiffLine, d2 *DiffLine) {
	if d1 != d2 {
		t.Errorf("%v should be equal %v", d1, d2)
	}
}

func TestDiffToHTML(t *testing.T) {
	assertEqual(t, "+foo <span class=\"added-code\">bar</span> biz", diffToHTML([]dmp.Diff{
		{dmp.DiffEqual, "foo "},
		{dmp.DiffInsert, "bar"},
		{dmp.DiffDelete, " baz"},
		{dmp.DiffEqual, " biz"},
	}, DiffLineAdd))

	assertEqual(t, "-foo <span class=\"removed-code\">bar</span> biz", diffToHTML([]dmp.Diff{
		{dmp.DiffEqual, "foo "},
		{dmp.DiffDelete, "bar"},
		{dmp.DiffInsert, " baz"},
		{dmp.DiffEqual, " biz"},
	}, DiffLineDel))
}
