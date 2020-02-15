package enry

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/src-d/enry/v2/data"
	"github.com/src-d/enry/v2/regex"
)

// OtherLanguage is used as a zero value when a function can not return a specific language.
const OtherLanguage = ""

// Strategy type fix the signature for the functions that can be used as a strategy.
type Strategy func(filename string, content []byte, candidates []string) (languages []string)

// DefaultStrategies is a sequence of strategies used by GetLanguage to detect languages.
var DefaultStrategies = []Strategy{
	GetLanguagesByModeline,
	GetLanguagesByFilename,
	GetLanguagesByShebang,
	GetLanguagesByExtension,
	GetLanguagesByContent,
	GetLanguagesByClassifier,
}

// DefaultClassifier is a Naive Bayes classifier trained on Linguist samples.
var DefaultClassifier Classifier = &classifier{
	languagesLogProbabilities: data.LanguagesLogProbabilities,
	tokensLogProbabilities:    data.TokensLogProbabilities,
	tokensTotal:               data.TokensTotal,
}

// GetLanguage applies a sequence of strategies based on the given filename and content
// to find out the most probably language to return.
func GetLanguage(filename string, content []byte) (language string) {
	languages := GetLanguages(filename, content)
	return firstLanguage(languages)
}

func firstLanguage(languages []string) string {
	for _, l := range languages {
		if l != "" {
			return l
		}
	}
	return OtherLanguage
}

// GetLanguageByModeline returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByModeline(content []byte) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByModeline, "", content, nil)
}

// GetLanguageByEmacsModeline returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByEmacsModeline(content []byte) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByEmacsModeline, "", content, nil)
}

// GetLanguageByVimModeline returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByVimModeline(content []byte) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByVimModeline, "", content, nil)
}

// GetLanguageByFilename returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByFilename(filename string) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByFilename, filename, nil, nil)
}

// GetLanguageByShebang returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByShebang(content []byte) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByShebang, "", content, nil)
}

// GetLanguageByExtension returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByExtension(filename string) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByExtension, filename, nil, nil)
}

// GetLanguageByContent returns detected language. If there are more than one possibles languages
// it returns the first language by alphabetically order and safe to false.
func GetLanguageByContent(filename string, content []byte) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByContent, filename, content, nil)
}

// GetLanguageByClassifier returns the most probably language detected for the given content. It uses
// DefaultClassifier, if no candidates are provided it returns OtherLanguage.
func GetLanguageByClassifier(content []byte, candidates []string) (language string, safe bool) {
	return getLanguageByStrategy(GetLanguagesByClassifier, "", content, candidates)
}

func getLanguageByStrategy(strategy Strategy, filename string, content []byte, candidates []string) (string, bool) {
	languages := strategy(filename, content, candidates)
	return getFirstLanguageAndSafe(languages)
}

func getFirstLanguageAndSafe(languages []string) (language string, safe bool) {
	language = firstLanguage(languages)
	safe = len(languages) == 1
	return
}

// GetLanguageBySpecificClassifier returns the most probably language for the given content using
// classifier to detect language.
func GetLanguageBySpecificClassifier(content []byte, candidates []string, classifier Classifier) (language string, safe bool) {
	languages := GetLanguagesBySpecificClassifier(content, candidates, classifier)
	return getFirstLanguageAndSafe(languages)
}

// GetLanguages applies a sequence of strategies based on the given filename and content
// to find out the most probably languages to return.
// At least one of arguments should be set. If content is missing, language detection will be based on the filename.
// The function won't read the file, given an empty content.
func GetLanguages(filename string, content []byte) []string {
	if IsBinary(content) {
		return nil
	}

	var languages []string
	candidates := []string{}
	for _, strategy := range DefaultStrategies {
		languages = strategy(filename, content, candidates)
		if len(languages) == 1 {
			return languages
		}

		if len(languages) > 0 {
			candidates = append(candidates, languages...)
		}
	}

	return languages
}

// GetLanguagesByModeline returns a slice of possible languages for the given content.
// It complies with the signature to be a Strategy type.
func GetLanguagesByModeline(_ string, content []byte, candidates []string) []string {
	headFoot := getHeaderAndFooter(content)
	var languages []string
	for _, getLang := range modelinesFunc {
		languages = getLang("", headFoot, candidates)
		if len(languages) > 0 {
			break
		}
	}

	return languages
}

var modelinesFunc = []Strategy{
	GetLanguagesByEmacsModeline,
	GetLanguagesByVimModeline,
}

func getHeaderAndFooter(content []byte) []byte {
	const searchScope = 5

	if len(content) == 0 {
		return content
	}

	if bytes.Count(content, []byte("\n")) < 2*searchScope {
		return content
	}

	header := headScope(content, searchScope)
	footer := footScope(content, searchScope)
	headerAndFooter := make([]byte, 0, len(content[:header])+len(content[footer:]))
	headerAndFooter = append(headerAndFooter, content[:header]...)
	headerAndFooter = append(headerAndFooter, content[footer:]...)
	return headerAndFooter
}

func headScope(content []byte, scope int) (index int) {
	for i := 0; i < scope; i++ {
		eol := bytes.IndexAny(content, "\n")
		content = content[eol+1:]
		index += eol
	}

	return index + scope - 1
}

func footScope(content []byte, scope int) (index int) {
	for i := 0; i < scope; i++ {
		index = bytes.LastIndexAny(content, "\n")
		content = content[:index]
	}

	return index + 1
}

var (
	reEmacsModeline = regex.MustCompile(`.*-\*-\s*(.+?)\s*-\*-.*(?m:$)`)
	reEmacsLang     = regex.MustCompile(`.*(?i:mode)\s*:\s*([^\s;]+)\s*;*.*`)
	reVimModeline   = regex.MustCompile(`(?:(?m:\s|^)vi(?:m[<=>]?\d+|m)?|[\t\x20]*ex)\s*[:]\s*(.*)(?m:$)`)
	reVimLang       = regex.MustCompile(`(?i:filetype|ft|syntax)\s*=(\w+)(?:\s|:|$)`)
)

// GetLanguagesByEmacsModeline returns a slice of possible languages for the given content.
// It complies with the signature to be a Strategy type.
func GetLanguagesByEmacsModeline(_ string, content []byte, _ []string) []string {
	matched := reEmacsModeline.FindAllSubmatch(content, -1)
	if matched == nil {
		return nil
	}

	// only take the last matched line, discard previous lines
	lastLineMatched := matched[len(matched)-1][1]
	matchedAlias := reEmacsLang.FindSubmatch(lastLineMatched)
	var alias string
	if matchedAlias != nil {
		alias = string(matchedAlias[1])
	} else {
		alias = string(lastLineMatched)
	}

	language, ok := GetLanguageByAlias(alias)
	if !ok {
		return nil
	}

	return []string{language}
}

// GetLanguagesByVimModeline returns a slice of possible languages for the given content.
// It complies with the signature to be a Strategy type.
func GetLanguagesByVimModeline(_ string, content []byte, _ []string) []string {
	matched := reVimModeline.FindAllSubmatch(content, -1)
	if matched == nil {
		return nil
	}

	// only take the last matched line, discard previous lines
	lastLineMatched := matched[len(matched)-1][1]
	matchedAlias := reVimLang.FindAllSubmatch(lastLineMatched, -1)
	if matchedAlias == nil {
		return nil
	}

	alias := string(matchedAlias[0][1])
	if len(matchedAlias) > 1 {
		// cases:
		// matchedAlias = [["syntax=ruby " "ruby"] ["ft=python " "python"] ["filetype=perl " "perl"]] returns OtherLanguage;
		// matchedAlias = [["syntax=python " "python"] ["ft=python " "python"] ["filetype=python " "python"]] returns "Python";
		for _, match := range matchedAlias {
			otherAlias := string(match[1])
			if otherAlias != alias {
				return nil
			}
		}
	}

	language, ok := GetLanguageByAlias(alias)
	if !ok {
		return nil
	}

	return []string{language}
}

// GetLanguagesByFilename returns a slice of possible languages for the given filename.
// It complies with the signature to be a Strategy type.
func GetLanguagesByFilename(filename string, _ []byte, _ []string) []string {
	if filename == "" {
		return nil
	}

	return data.LanguagesByFilename[filepath.Base(filename)]
}

// GetLanguagesByShebang returns a slice of possible languages for the given content.
// It complies with the signature to be a Strategy type.
func GetLanguagesByShebang(_ string, content []byte, _ []string) (languages []string) {
	interpreter := getInterpreter(content)
	return data.LanguagesByInterpreter[interpreter]
}

var (
	shebangExecHack = regex.MustCompile(`exec (\w+).+\$0.+\$@`)
	pythonVersion   = regex.MustCompile(`python\d\.\d+`)
)

func getInterpreter(data []byte) (interpreter string) {
	line := getFirstLine(data)
	if !hasShebang(line) {
		return ""
	}

	// skip shebang
	line = bytes.TrimSpace(line[2:])
	splitted := bytes.Fields(line)
	if len(splitted) == 0 {
		return ""
	}

	if bytes.Contains(splitted[0], []byte("env")) {
		if len(splitted) > 1 {
			interpreter = string(splitted[1])
		}
	} else {
		splittedPath := bytes.Split(splitted[0], []byte{'/'})
		interpreter = string(splittedPath[len(splittedPath)-1])
	}

	if interpreter == "sh" {
		interpreter = lookForMultilineExec(data)
	}

	if pythonVersion.MatchString(interpreter) {
		interpreter = interpreter[:strings.Index(interpreter, `.`)]
	}

	// If osascript is called with argument -l it could be different language so do not relay on it
	// To match linguist behaviour, see ref https://github.com/github/linguist/blob/d95bae794576ab0ef2fcb41a39eb61ea5302c5b5/lib/linguist/shebang.rb#L63
	if interpreter == "osascript" && bytes.Contains(line, []byte("-l")) {
		interpreter = ""
	}

	return
}

func getFirstLine(data []byte) []byte {
	buf := bufio.NewScanner(bytes.NewReader(data))
	buf.Scan()
	line := buf.Bytes()
	if err := buf.Err(); err != nil {
		return nil
	}

	return line
}

func hasShebang(line []byte) bool {
	const shebang = `#!`
	prefix := []byte(shebang)
	return bytes.HasPrefix(line, prefix)
}

func lookForMultilineExec(data []byte) string {
	const magicNumOfLines = 5
	interpreter := "sh"

	buf := bufio.NewScanner(bytes.NewReader(data))
	for i := 0; i < magicNumOfLines && buf.Scan(); i++ {
		line := buf.Bytes()
		if shebangExecHack.Match(line) {
			interpreter = shebangExecHack.FindStringSubmatch(string(line))[1]
			break
		}
	}

	if err := buf.Err(); err != nil {
		return interpreter
	}

	return interpreter
}

// GetLanguagesByExtension returns a slice of possible languages for the given filename.
// It complies with the signature to be a Strategy type.
func GetLanguagesByExtension(filename string, _ []byte, _ []string) []string {
	if !strings.Contains(filename, ".") {
		return nil
	}

	filename = strings.ToLower(filename)
	dots := getDotIndexes(filename)
	for _, dot := range dots {
		ext := filename[dot:]
		languages, ok := data.LanguagesByExtension[ext]
		if ok {
			return languages
		}
	}

	return nil
}

func getDotIndexes(filename string) []int {
	dots := make([]int, 0, 2)
	for i, letter := range filename {
		if letter == rune('.') {
			dots = append(dots, i)
		}
	}

	return dots
}

// GetLanguagesByContent returns a slice of languages for the given content.
// It is a Strategy that uses content-based regexp heuristics and a filename extension.
func GetLanguagesByContent(filename string, content []byte, _ []string) []string {
	if filename == "" {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filename))

	heuristic, ok := data.ContentHeuristics[ext]
	if !ok {
		return nil
	}

	return heuristic.Match(content)
}

// GetLanguagesByClassifier uses DefaultClassifier as a Classifier and returns a sorted slice of possible languages ordered by
// decreasing language's probability. If there are not candidates it returns nil. It complies with the signature to be a Strategy type.
func GetLanguagesByClassifier(filename string, content []byte, candidates []string) (languages []string) {
	if len(candidates) == 0 {
		return nil
	}

	return GetLanguagesBySpecificClassifier(content, candidates, DefaultClassifier)
}

// GetLanguagesBySpecificClassifier returns a slice of possible languages. It takes in a Classifier to be used.
func GetLanguagesBySpecificClassifier(content []byte, candidates []string, classifier Classifier) (languages []string) {
	mapCandidates := make(map[string]float64)
	for _, candidate := range candidates {
		mapCandidates[candidate]++
	}

	return classifier.Classify(content, mapCandidates)
}

// GetLanguageExtensions returns the different extensions being used by the language.
func GetLanguageExtensions(language string) []string {
	return data.ExtensionsByLanguage[language]
}

// Type represent language's type. Either data, programming, markup, prose, or unknown.
type Type int

// Type's values.
const (
	Unknown Type = iota
	Data
	Programming
	Markup
	Prose
)

// GetLanguageType returns the type of the given language.
func GetLanguageType(language string) (langType Type) {
	intType, ok := data.LanguagesType[language]
	langType = Type(intType)
	if !ok {
		langType = Unknown
	}
	return langType
}

// GetLanguageByAlias returns either the language related to the given alias and ok set to true
// or Otherlanguage and ok set to false if the alias is not recognized.
func GetLanguageByAlias(alias string) (lang string, ok bool) {
	lang, ok = data.LanguageByAlias(alias)
	if !ok {
		lang = OtherLanguage
	}

	return
}
