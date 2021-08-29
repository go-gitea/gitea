package enry

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2/data"
	"github.com/go-enry/go-enry/v2/regex"
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
	GetLanguagesByXML,
	GetLanguagesByManpage,
	GetLanguagesByContent,
	GetLanguagesByClassifier,
}

// defaultClassifier is a Naive Bayes classifier trained on Linguist samples.
var defaultClassifier classifier = &naiveBayes{
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
// defaultClassifier, if no candidates are provided it returns OtherLanguage.
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

// getLanguageBySpecificClassifier returns the most probably language for the given content using
// classifier to detect language.
func getLanguageBySpecificClassifier(content []byte, candidates []string, classifier classifier) (language string, safe bool) {
	languages := getLanguagesBySpecificClassifier(content, candidates, classifier)
	return getFirstLanguageAndSafe(languages)
}

// GetLanguages applies a sequence of strategies based on the given filename and content
// to find out the most probable languages to return.
//
// If it finds a strategy that produces a single result, it will be returned;
// otherise the last strategy that returned multiple results will be returned.
// If the content is binary, no results will be returned. This matches the
// behavior of Linguist.detect: https://github.com/github/linguist/blob/aad49acc0624c70d654a8dce447887dbbc713c7a/lib/linguist.rb#L14-L49
//
// At least one of arguments should be set. If content is missing, language detection will be based on the filename.
// The function won't read the file, given an empty content.
func GetLanguages(filename string, content []byte) []string {
	if IsBinary(content) {
		return nil
	}

	var languages []string
	for _, strategy := range DefaultStrategies {
		candidates := strategy(filename, content, languages)
		// No candidates, continue to next strategy without updating languages
		if len(candidates) == 0 {
			continue
		}

		// Only one candidate match, return it
		if len(candidates) == 1 {
			return candidates
		}

		// Save the candidates from this strategy to pass onto to the next strategy, like Linguist
		languages = candidates
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

func getFirstLines(content []byte, count int) []byte {
	nlpos := -1
	for ; count > 0; count-- {
		pos := bytes.IndexByte(content[nlpos+1:], '\n')
		if pos < 0 {
			return content
		}
		nlpos += pos + 1
	}

	return content[:nlpos]
}

func getFirstLine(content []byte) []byte {
	return getFirstLines(content, 1)
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

var (
	manpageExtension = regex.MustCompile(`\.(?:[1-9](?:[a-z_]+[a-z_0-9]*)?|0p|n|man|mdoc)(?:\.in)?$`)
)

// GetLanguagesByManpage returns a slice of possible manpage languages for the given filename.
// It complies with the signature to be a Strategy type.
func GetLanguagesByManpage(filename string, _ []byte, _ []string) []string {
	filename = strings.ToLower(filename)

	// Check if matches Roff man page filenames
	if manpageExtension.Match([]byte(filename)) {
		return []string{
			"Roff Manpage",
			"Roff",
		}
	}

	return nil
}

var (
	xmlHeader = regex.MustCompile(`<?xml version=`)
)

// GetLanguagesByXML returns a slice of possible XML language for the given filename.
// It complies with the signature to be a Strategy type.
func GetLanguagesByXML(_ string, content []byte, candidates []string) []string {
	if len(candidates) > 0 {
		return candidates
	}

	header := getFirstLines(content, 2)

	// Check if contains XML header
	if xmlHeader.Match(header) {
		return []string{
			"XML",
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

// GetLanguagesByClassifier returns a sorted slice of possible languages ordered by
// decreasing language's probability. If there are not candidates it returns nil.
// It is a Strategy that uses a pre-trained defaultClassifier.
func GetLanguagesByClassifier(filename string, content []byte, candidates []string) (languages []string) {
	if len(candidates) == 0 {
		return nil
	}

	return getLanguagesBySpecificClassifier(content, candidates, defaultClassifier)
}

// getLanguagesBySpecificClassifier returns a slice of possible languages. It takes in a Classifier to be used.
func getLanguagesBySpecificClassifier(content []byte, candidates []string, classifier classifier) (languages []string) {
	mapCandidates := make(map[string]float64)
	for _, candidate := range candidates {
		mapCandidates[candidate]++
	}

	return classifier.classify(content, mapCandidates)
}

// GetLanguageExtensions returns all extensions associated with the given language.
func GetLanguageExtensions(language string) []string {
	return data.ExtensionsByLanguage[language]
}

// GetLanguageID returns the ID for the language. IDs are assigned by GitHub.
// The input must be the canonical language name. Aliases are not supported.
//
// NOTE: The zero value (0) is a valid language ID, so this API mimics the Go
// map API. Use the second return value to check if the language was found.
func GetLanguageID(language string) (int, bool) {
	id, ok := data.IDByLanguage[language]
	return id, ok
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

// GetLanguageGroup returns language group or empty string if language does not have group.
func GetLanguageGroup(language string) string {
	if group, ok := data.LanguagesGroup[language]; ok {
		return group
	}

	return ""
}
