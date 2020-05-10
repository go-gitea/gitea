package substring

import (
	"bytes"
	"regexp"

	"github.com/toqueteos/trie"
)

type BytesMatcher interface {
	Match(b []byte) bool
	MatchIndex(b []byte) int
}

// regexp
type regexpBytes struct{ re *regexp.Regexp }

func BytesRegexp(pat string) *regexpBytes  { return &regexpBytes{regexp.MustCompile(pat)} }
func (m *regexpBytes) Match(b []byte) bool { return m.re.Match(b) }
func (m *regexpBytes) MatchIndex(b []byte) int {
	found := m.re.FindIndex(b)
	if found != nil {
		return found[1]
	}
	return -1
}

// exact
type exactBytes struct{ pat []byte }

func BytesExact(pat string) *exactBytes { return &exactBytes{[]byte(pat)} }
func (m *exactBytes) Match(b []byte) bool {
	l, r := len(m.pat), len(b)
	if l != r {
		return false
	}
	for i := 0; i < l; i++ {
		if b[i] != m.pat[i] {
			return false
		}
	}
	return true
}
func (m *exactBytes) MatchIndex(b []byte) int {
	if m.Match(b) {
		return len(b)
	}
	return -1
}

// any, search `s` in `.Match(pat)`
type anyBytes struct {
	pat []byte
}

func BytesAny(pat string) *anyBytes     { return &anyBytes{[]byte(pat)} }
func (m *anyBytes) Match(b []byte) bool { return bytes.Index(m.pat, b) >= 0 }
func (m *anyBytes) MatchIndex(b []byte) int {
	if idx := bytes.Index(m.pat, b); idx >= 0 {
		return idx + len(b)
	}
	return -1
}

// has, search `pat` in `.Match(s)`
type hasBytes struct {
	pat []byte
}

func BytesHas(pat string) *hasBytes     { return &hasBytes{[]byte(pat)} }
func (m *hasBytes) Match(b []byte) bool { return bytes.Index(b, m.pat) >= 0 }
func (m *hasBytes) MatchIndex(b []byte) int {
	if idx := bytes.Index(b, m.pat); idx >= 0 {
		return idx + len(m.pat)
	}
	return -1
}

// prefix
type prefixBytes struct{ pat []byte }

func BytesPrefix(pat string) *prefixBytes  { return &prefixBytes{[]byte(pat)} }
func (m *prefixBytes) Match(b []byte) bool { return bytes.HasPrefix(b, m.pat) }
func (m *prefixBytes) MatchIndex(b []byte) int {
	if bytes.HasPrefix(b, m.pat) {
		return len(m.pat)
	}
	return -1
}

// prefixes
type prefixesBytes struct {
	t *trie.Trie
}

func BytesPrefixes(pats ...string) *prefixesBytes {
	t := trie.New()
	for _, pat := range pats {
		t.Insert([]byte(pat))
	}
	return &prefixesBytes{t}
}
func (m *prefixesBytes) Match(b []byte) bool { return m.t.PrefixIndex(b) >= 0 }
func (m *prefixesBytes) MatchIndex(b []byte) int {
	if idx := m.t.PrefixIndex(b); idx >= 0 {
		return idx
	}
	return -1
}

// suffix
type suffixBytes struct{ pat []byte }

func BytesSuffix(pat string) *suffixBytes  { return &suffixBytes{[]byte(pat)} }
func (m *suffixBytes) Match(b []byte) bool { return bytes.HasSuffix(b, m.pat) }
func (m *suffixBytes) MatchIndex(b []byte) int {
	if bytes.HasSuffix(b, m.pat) {
		return len(m.pat)
	}
	return -1
}

// suffixes
type suffixesBytes struct {
	t *trie.Trie
}

func BytesSuffixes(pats ...string) *suffixesBytes {
	t := trie.New()
	for _, pat := range pats {
		t.Insert(reverse([]byte(pat)))
	}
	return &suffixesBytes{t}
}
func (m *suffixesBytes) Match(b []byte) bool {
	return m.t.PrefixIndex(reverse(b)) >= 0
}
func (m *suffixesBytes) MatchIndex(b []byte) int {
	if idx := m.t.PrefixIndex(reverse(b)); idx >= 0 {
		return idx
	}
	return -1
}

// after
type afterBytes struct {
	first   []byte
	matcher BytesMatcher
}

func BytesAfter(first string, m BytesMatcher) *afterBytes { return &afterBytes{[]byte(first), m} }
func (a *afterBytes) Match(b []byte) bool {
	if idx := bytes.Index(b, a.first); idx >= 0 {
		return a.matcher.Match(b[idx+len(a.first):])
	}
	return false
}
func (a *afterBytes) MatchIndex(b []byte) int {
	if idx := bytes.Index(b, a.first); idx >= 0 {
		return idx + a.matcher.MatchIndex(b[idx:])
	}
	return -1
}

// and, returns true iff all matchers return true
type andBytes struct{ matchers []BytesMatcher }

func BytesAnd(m ...BytesMatcher) *andBytes { return &andBytes{m} }
func (a *andBytes) Match(b []byte) bool {
	for _, m := range a.matchers {
		if !m.Match(b) {
			return false
		}
	}
	return true
}
func (a *andBytes) MatchIndex(b []byte) int {
	longest := 0
	for _, m := range a.matchers {
		if idx := m.MatchIndex(b); idx < 0 {
			return -1
		} else if idx > longest {
			longest = idx
		}
	}
	return longest
}

// or, returns true iff any matcher returns true
type orBytes struct{ matchers []BytesMatcher }

func BytesOr(m ...BytesMatcher) *orBytes { return &orBytes{m} }
func (o *orBytes) Match(b []byte) bool {
	for _, m := range o.matchers {
		if m.Match(b) {
			return true
		}
	}
	return false
}
func (o *orBytes) MatchIndex(b []byte) int {
	for _, m := range o.matchers {
		if idx := m.MatchIndex(b); idx >= 0 {
			return idx
		}
	}
	return -1
}

type suffixGroupBytes struct {
	suffix   BytesMatcher
	matchers []BytesMatcher
}

func BytesSuffixGroup(s string, m ...BytesMatcher) *suffixGroupBytes {
	return &suffixGroupBytes{BytesSuffix(s), m}
}
func (sg *suffixGroupBytes) Match(b []byte) bool {
	if sg.suffix.Match(b) {
		return BytesOr(sg.matchers...).Match(b)
	}
	return false
}
func (sg *suffixGroupBytes) MatchIndex(b []byte) int {
	if sg.suffix.MatchIndex(b) >= 0 {
		return BytesOr(sg.matchers...).MatchIndex(b)
	}
	return -1
}
