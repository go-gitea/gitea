package enry

import (
	"math"
	"sort"

	"github.com/src-d/enry/v2/internal/tokenizer"
)

// Classifier is the interface in charge to detect the possible languages of the given content based on a set of
// candidates. Candidates is a map which can be used to assign weights to languages dynamically.
type Classifier interface {
	Classify(content []byte, candidates map[string]float64) (languages []string)
}

type classifier struct {
	languagesLogProbabilities map[string]float64
	tokensLogProbabilities    map[string]map[string]float64
	tokensTotal               float64
}

type scoredLanguage struct {
	language string
	score    float64
}

// Classify returns a sorted slice of possible languages sorted by decreasing language's probability
func (c *classifier) Classify(content []byte, candidates map[string]float64) []string {

	var languages map[string]float64
	if len(candidates) == 0 {
		languages = c.knownLangs()
	} else {
		languages = make(map[string]float64, len(candidates))
		for candidate, weight := range candidates {
			if lang, ok := GetLanguageByAlias(candidate); ok {
				candidate = lang
			}

			languages[candidate] = weight
		}
	}

	empty := len(content) == 0
	scoredLangs := make([]*scoredLanguage, 0, len(languages))

	var tokens []string
	if !empty {
		tokens = tokenizer.Tokenize(content)
	}

	for language := range languages {
		score := c.languagesLogProbabilities[language]
		if !empty {
			score += c.tokensLogProbability(tokens, language)
		}
		scoredLangs = append(scoredLangs, &scoredLanguage{
			language: language,
			score:    score,
		})
	}

	return sortLanguagesByScore(scoredLangs)
}

func sortLanguagesByScore(scoredLangs []*scoredLanguage) []string {
	sort.Stable(byScore(scoredLangs))
	sortedLanguages := make([]string, 0, len(scoredLangs))
	for _, scoredLang := range scoredLangs {
		sortedLanguages = append(sortedLanguages, scoredLang.language)
	}

	return sortedLanguages
}

func (c *classifier) knownLangs() map[string]float64 {
	langs := make(map[string]float64, len(c.languagesLogProbabilities))
	for lang := range c.languagesLogProbabilities {
		langs[lang]++
	}

	return langs
}

func (c *classifier) tokensLogProbability(tokens []string, language string) float64 {
	var sum float64
	for _, token := range tokens {
		sum += c.tokenProbability(token, language)
	}

	return sum
}

func (c *classifier) tokenProbability(token, language string) float64 {
	tokenProb, ok := c.tokensLogProbabilities[language][token]
	if !ok {
		tokenProb = math.Log(1.000000 / c.tokensTotal)
	}

	return tokenProb
}

type byScore []*scoredLanguage

func (b byScore) Len() int           { return len(b) }
func (b byScore) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byScore) Less(i, j int) bool { return b[j].score < b[i].score }
