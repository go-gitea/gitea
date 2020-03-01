package data

import "github.com/src-d/enry/v2/data/rule"

// Heuristics implements a rule-based content matching engine.

// Heuristics is a number of sequntially applied rule.Heuristic where a
// matching one disambiguages language(s) for a single file extension.
type Heuristics []rule.Heuristic

// Match returns languages identified by the matching rule of the heuristic.
func (hs Heuristics) Match(data []byte) []string {
	var matchedLangs []string
	for _, heuristic := range hs {
		if heuristic.Match(data) {
			for _, langOrAlias := range heuristic.Languages() {
				lang, ok := LanguageByAlias(langOrAlias)
				if !ok { // should never happen
					// reaching here means language name/alias in heuristics.yml
					// is not consistent with languages.yml
					// but we do not surface any such error at the API
					continue
				}
				matchedLangs = append(matchedLangs, lang)
			}
			break
		}
	}
	return matchedLangs
}

// matchString is a convenience used only in tests.
func (hs *Heuristics) matchString(data string) []string {
	return hs.Match([]byte(data))
}
