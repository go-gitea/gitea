package substring

import (
	"regexp"
	"strings"

	"github.com/toqueteos/trie"
)

type StringsMatcher interface {
	Match(s string) bool
	MatchIndex(s string) int
}

// regexp
type regexpString struct{ re *regexp.Regexp }

func Regexp(pat string) *regexpString       { return &regexpString{regexp.MustCompile(pat)} }
func (m *regexpString) Match(s string) bool { return m.re.MatchString(s) }
func (m *regexpString) MatchIndex(s string) int {
	found := m.re.FindStringIndex(s)
	if found != nil {
		return found[1]
	}
	return -1
}

// exact
type exactString struct{ pat string }

func Exact(pat string) *exactString        { return &exactString{pat} }
func (m *exactString) Match(s string) bool { return m.pat == s }
func (m *exactString) MatchIndex(s string) int {
	if m.pat == s {
		return len(s)
	}
	return -1
}

// any, search `s` in `.Match(pat)`
type anyString struct{ pat string }

func Any(pat string) *anyString { return &anyString{pat} }
func (m *anyString) Match(s string) bool {
	return strings.Index(m.pat, s) >= 0
}
func (m *anyString) MatchIndex(s string) int {
	if idx := strings.Index(m.pat, s); idx >= 0 {
		return idx + len(s)
	}
	return -1
}

// has, search `pat` in `.Match(s)`
type hasString struct{ pat string }

func Has(pat string) *hasString { return &hasString{pat} }
func (m *hasString) Match(s string) bool {
	return strings.Index(s, m.pat) >= 0
}
func (m *hasString) MatchIndex(s string) int {
	if idx := strings.Index(s, m.pat); idx >= 0 {
		return idx + len(m.pat)
	}
	return -1
}

// prefix
type prefixString struct{ pat string }

func Prefix(pat string) *prefixString       { return &prefixString{pat} }
func (m *prefixString) Match(s string) bool { return strings.HasPrefix(s, m.pat) }
func (m *prefixString) MatchIndex(s string) int {
	if strings.HasPrefix(s, m.pat) {
		return len(m.pat)
	}
	return -1
}

// prefixes
type prefixesString struct{ t *trie.Trie }

func Prefixes(pats ...string) *prefixesString {
	t := trie.New()
	for _, pat := range pats {
		t.Insert([]byte(pat))
	}
	return &prefixesString{t}
}
func (m *prefixesString) Match(s string) bool { return m.t.PrefixIndex([]byte(s)) >= 0 }
func (m *prefixesString) MatchIndex(s string) int {
	if idx := m.t.PrefixIndex([]byte(s)); idx >= 0 {
		return idx
	}
	return -1
}

// suffix
type suffixString struct{ pat string }

func Suffix(pat string) *suffixString       { return &suffixString{pat} }
func (m *suffixString) Match(s string) bool { return strings.HasSuffix(s, m.pat) }
func (m *suffixString) MatchIndex(s string) int {
	if strings.HasSuffix(s, m.pat) {
		return len(m.pat)
	}
	return -1
}

// suffixes
type suffixesString struct{ t *trie.Trie }

func Suffixes(pats ...string) *suffixesString {
	t := trie.New()
	for _, pat := range pats {
		t.Insert(reverse([]byte(pat)))
	}
	return &suffixesString{t}
}
func (m *suffixesString) Match(s string) bool {
	return m.t.PrefixIndex(reverse([]byte(s))) >= 0
}
func (m *suffixesString) MatchIndex(s string) int {
	if idx := m.t.PrefixIndex(reverse([]byte(s))); idx >= 0 {
		return idx
	}
	return -1
}

// after
type afterString struct {
	first   string
	matcher StringsMatcher
}

func After(first string, m StringsMatcher) *afterString {
	return &afterString{first, m}
}
func (a *afterString) Match(s string) bool {
	if idx := strings.Index(s, a.first); idx >= 0 {
		return a.matcher.Match(s[idx+len(a.first):])
	}
	return false
}
func (a *afterString) MatchIndex(s string) int {
	if idx := strings.Index(s, a.first); idx >= 0 {
		return idx + a.matcher.MatchIndex(s[idx+len(a.first):])
	}
	return -1
}

// and, returns true iff all matchers return true
type andString struct{ matchers []StringsMatcher }

func And(m ...StringsMatcher) *andString { return &andString{m} }
func (a *andString) Match(s string) bool {
	for _, m := range a.matchers {
		if !m.Match(s) {
			return false
		}
	}
	return true
}
func (a *andString) MatchIndex(s string) int {
	longest := 0
	for _, m := range a.matchers {
		if idx := m.MatchIndex(s); idx < 0 {
			return -1
		} else if idx > longest {
			longest = idx
		}
	}
	return longest
}

// or, returns true iff any matcher returns true
type orString struct{ matchers []StringsMatcher }

func Or(m ...StringsMatcher) *orString { return &orString{m} }
func (o *orString) Match(s string) bool {
	for _, m := range o.matchers {
		if m.Match(s) {
			return true
		}
	}
	return false
}
func (o *orString) MatchIndex(s string) int {
	for _, m := range o.matchers {
		if idx := m.MatchIndex(s); idx >= 0 {
			return idx
		}
	}
	return -1
}

type suffixGroupString struct {
	suffix   StringsMatcher
	matchers []StringsMatcher
}

func SuffixGroup(s string, m ...StringsMatcher) *suffixGroupString {
	return &suffixGroupString{Suffix(s), m}
}
func (sg *suffixGroupString) Match(s string) bool {
	if sg.suffix.Match(s) {
		return Or(sg.matchers...).Match(s)
	}
	return false
}
func (sg *suffixGroupString) MatchIndex(s string) int {
	if sg.suffix.MatchIndex(s) >= 0 {
		return Or(sg.matchers...).MatchIndex(s)
	}
	return -1
}
