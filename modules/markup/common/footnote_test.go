package common

import "testing"

func TestCleanValue(t *testing.T) {
	var tests = []struct {
		param  string
		expect string
	}{
		// Github behavior test cases
		{"", ""},
		{"test(0)", "test0"},
		{"test!1", "test1"},
		{"test:2", "test2"},
		{"test*3", "test3"},
		{"test！4", "test4"},
		{"test：5", "test5"},
		{"test*6", "test6"},
		{"test：6 a", "test6-a"},
		{"test：6 !b", "test6-b"},
		{"test：ad # df", "testad--df"},
		{"test：ad #23 df 2*/*", "testad-23-df-2"},
		{"test：ad 23 df 2*/*", "testad-23-df-2"},
		{"test：ad # 23 df 2*/*", "testad--23-df-2"},
		{"Anchors in Markdown", "anchors-in-markdown"},
		{"a_b_c", "a_b_c"},
		{"a-b-c", "a-b-c"},
		{"a-b-c----", "a-b-c----"},
		{"test：6a", "test6a"},
		{"test：a6", "testa6"},
		{"tes a a   a  a", "tes-a-a---a--a"},
		{"  tes a a   a  a  ", "tes-a-a---a--a"},
		{"Header with \"double quotes\"", "header-with-double-quotes"},
		{"Placeholder to force scrolling on link's click", "placeholder-to-force-scrolling-on-links-click"},
		{"Placeholder to force scrolling on link's click", "placeholder-to-force-scrolling-on-links-click"},
	}
	for _, test := range tests {
		if got := CleanValue([]byte(test.param)); string(got) != test.expect {
			t.Errorf("CleanValue(%q) = %q, want %q", test.param, got, test.expect)
		}
	}
}
