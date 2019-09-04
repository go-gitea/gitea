package codescan

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-openapi/loads/fmts"
	"github.com/go-openapi/spec"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func shouldAcceptTag(tags []string, includeTags map[string]bool, excludeTags map[string]bool) bool {
	for _, tag := range tags {
		if len(includeTags) > 0 {
			if includeTags[tag] {
				return true
			}
		} else if len(excludeTags) > 0 {
			if excludeTags[tag] {
				return false
			}
		}
	}
	return len(includeTags) == 0
}

func shouldAcceptPkg(path string, includePkgs, excludePkgs []string) bool {
	if len(includePkgs) == 0 && len(excludePkgs) == 0 {
		return true
	}
	for _, pkgName := range includePkgs {
		matched, _ := regexp.MatchString(pkgName, path)
		if matched {
			return true
		}
	}
	for _, pkgName := range excludePkgs {
		matched, _ := regexp.MatchString(pkgName, path)
		if matched {
			return false
		}
	}
	return len(includePkgs) == 0
}

// Many thanks go to https://github.com/yvasiyarov/swagger
// this is loosely based on that implementation but for swagger 2.0

func joinDropLast(lines []string) string {
	l := len(lines)
	lns := lines
	if l > 0 && len(strings.TrimSpace(lines[l-1])) == 0 {
		lns = lines[:l-1]
	}
	return strings.Join(lns, "\n")
}

func removeEmptyLines(lines []string) (notEmpty []string) {
	for _, l := range lines {
		if len(strings.TrimSpace(l)) > 0 {
			notEmpty = append(notEmpty, l)
		}
	}
	return
}

func rxf(rxp, ar string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(rxp, ar))
}

func allOfMember(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxAllOf.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func fileParam(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxFileUpload.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func strfmtName(comments *ast.CommentGroup) (string, bool) {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxStrFmt.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}
	}
	return "", false
}

func ignored(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxIgnoreOverride.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func enumName(comments *ast.CommentGroup) (string, bool) {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxEnum.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}
	}
	return "", false
}

func aliasParam(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxAlias.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func isAliasParam(prop swaggerTypable) bool {
	var isParam bool
	if param, ok := prop.(paramTypable); ok {
		isParam = param.param.In == "query" ||
			param.param.In == "path" ||
			param.param.In == "formData"
	}
	return isParam
}

func defaultName(comments *ast.CommentGroup) (string, bool) {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxDefault.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}
	}
	return "", false
}

func typeName(comments *ast.CommentGroup) (string, bool) {
	var typ string
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxType.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					typ = strings.TrimSpace(matches[1])
					return typ, true
				}
			}
		}
	}
	return "", false
}

type swaggerTypable interface {
	Typed(string, string)
	SetRef(spec.Ref)
	Items() swaggerTypable
	Schema() *spec.Schema
	Level() int
	AddExtension(key string, value interface{})
}

// Map all Go builtin types that have Json representation to Swagger/Json types.
// See https://golang.org/pkg/builtin/ and http://swagger.io/specification/
func swaggerSchemaForType(typeName string, prop swaggerTypable) error {
	switch typeName {
	case "bool":
		prop.Typed("boolean", "")
	case "byte":
		prop.Typed("integer", "uint8")
	case "complex128", "complex64":
		return fmt.Errorf("unsupported builtin %q (no JSON marshaller)", typeName)
	case "error":
		// TODO: error is often marshalled into a string but not always (e.g. errors package creates
		// errors that are marshalled into an empty object), this could be handled the same way
		// custom JSON marshallers are handled (in future)
		prop.Typed("string", "")
	case "float32":
		prop.Typed("number", "float")
	case "float64":
		prop.Typed("number", "double")
	case "int":
		prop.Typed("integer", "int64")
	case "int16":
		prop.Typed("integer", "int16")
	case "int32":
		prop.Typed("integer", "int32")
	case "int64":
		prop.Typed("integer", "int64")
	case "int8":
		prop.Typed("integer", "int8")
	case "rune":
		prop.Typed("integer", "int32")
	case "string":
		prop.Typed("string", "")
	case "uint":
		prop.Typed("integer", "uint64")
	case "uint16":
		prop.Typed("integer", "uint16")
	case "uint32":
		prop.Typed("integer", "uint32")
	case "uint64":
		prop.Typed("integer", "uint64")
	case "uint8":
		prop.Typed("integer", "uint8")
	case "uintptr":
		prop.Typed("integer", "uint64")
	default:
		return fmt.Errorf("unsupported type %q", typeName)
	}
	return nil
}

func newMultiLineTagParser(name string, parser valueParser, skipCleanUp bool) tagParser {
	return tagParser{
		Name:        name,
		MultiLine:   true,
		SkipCleanUp: skipCleanUp,
		Parser:      parser,
	}
}

func newSingleLineTagParser(name string, parser valueParser) tagParser {
	return tagParser{
		Name:        name,
		MultiLine:   false,
		SkipCleanUp: false,
		Parser:      parser,
	}
}

type tagParser struct {
	Name        string
	MultiLine   bool
	SkipCleanUp bool
	Lines       []string
	Parser      valueParser
}

func (st *tagParser) Matches(line string) bool {
	return st.Parser.Matches(line)
}

func (st *tagParser) Parse(lines []string) error {
	return st.Parser.Parse(lines)
}

func newYamlParser(rx *regexp.Regexp, setter func(json.RawMessage) error) valueParser {
	return &yamlParser{
		set: setter,
		rx:  rx,
	}
}

type yamlParser struct {
	set func(json.RawMessage) error
	rx  *regexp.Regexp
}

func (y *yamlParser) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var uncommented []string
	uncommented = append(uncommented, removeYamlIndent(lines)...)

	yamlContent := strings.Join(uncommented, "\n")
	var yamlValue interface{}
	err := yaml.Unmarshal([]byte(yamlContent), &yamlValue)
	if err != nil {
		return err
	}

	var jsonValue json.RawMessage
	jsonValue, err = fmts.YAMLToJSON(yamlValue)
	if err != nil {
		return err
	}

	return y.set(jsonValue)
}

func (y *yamlParser) Matches(line string) bool {
	return y.rx.MatchString(line)
}

// aggregates lines in header until it sees `---`,
// the beginning of a YAML spec
type yamlSpecScanner struct {
	header         []string
	yamlSpec       []string
	setTitle       func([]string)
	setDescription func([]string)
	workedOutTitle bool
	title          []string
	skipHeader     bool
}

func cleanupScannerLines(lines []string, ur *regexp.Regexp, yamlBlock *regexp.Regexp) []string {
	// bail early when there is nothing to parse
	if len(lines) == 0 {
		return lines
	}
	seenLine := -1
	var lastContent int
	var uncommented []string
	var startBlock bool
	var yamlLines []string
	for i, v := range lines {
		if yamlBlock != nil && yamlBlock.MatchString(v) && !startBlock {
			startBlock = true
			if seenLine < 0 {
				seenLine = i
			}
			continue
		}
		if startBlock {
			if yamlBlock != nil && yamlBlock.MatchString(v) {
				startBlock = false
				uncommented = append(uncommented, removeIndent(yamlLines)...)
				continue
			}
			yamlLines = append(yamlLines, v)
			if v != "" {
				if seenLine < 0 {
					seenLine = i
				}
				lastContent = i
			}
			continue
		}
		str := ur.ReplaceAllString(v, "")
		uncommented = append(uncommented, str)
		if str != "" {
			if seenLine < 0 {
				seenLine = i
			}
			lastContent = i
		}
	}

	// fixes issue #50
	if seenLine == -1 {
		return nil
	}
	return uncommented[seenLine : lastContent+1]
}

// a shared function that can be used to split given headers
// into a title and description
func collectScannerTitleDescription(headers []string) (title, desc []string) {
	hdrs := cleanupScannerLines(headers, rxUncommentHeaders, nil)

	idx := -1
	for i, line := range hdrs {
		if strings.TrimSpace(line) == "" {
			idx = i
			break
		}
	}

	if idx > -1 {
		title = hdrs[:idx]
		if len(hdrs) > idx+1 {
			desc = hdrs[idx+1:]
		} else {
			desc = nil
		}
		return
	}

	if len(hdrs) > 0 {
		line := hdrs[0]
		if rxPunctuationEnd.MatchString(line) {
			title = []string{line}
			desc = hdrs[1:]
		} else {
			desc = hdrs
		}
	}

	return
}

func (sp *yamlSpecScanner) collectTitleDescription() {
	if sp.workedOutTitle {
		return
	}
	if sp.setTitle == nil {
		sp.header = cleanupScannerLines(sp.header, rxUncommentHeaders, nil)
		return
	}

	sp.workedOutTitle = true
	sp.title, sp.header = collectScannerTitleDescription(sp.header)
}

func (sp *yamlSpecScanner) Title() []string {
	sp.collectTitleDescription()
	return sp.title
}

func (sp *yamlSpecScanner) Description() []string {
	sp.collectTitleDescription()
	return sp.header
}

func (sp *yamlSpecScanner) Parse(doc *ast.CommentGroup) error {
	if doc == nil {
		return nil
	}
	var startedYAMLSpec bool
COMMENTS:
	for _, c := range doc.List {
		for _, line := range strings.Split(c.Text, "\n") {
			if rxSwaggerAnnotation.MatchString(line) {
				break COMMENTS // a new swagger: annotation terminates this parser
			}

			if !startedYAMLSpec {
				if rxBeginYAMLSpec.MatchString(line) {
					startedYAMLSpec = true
					sp.yamlSpec = append(sp.yamlSpec, line)
					continue
				}

				if !sp.skipHeader {
					sp.header = append(sp.header, line)
				}

				// no YAML spec yet, moving on
				continue
			}

			sp.yamlSpec = append(sp.yamlSpec, line)
		}
	}
	if sp.setTitle != nil {
		sp.setTitle(sp.Title())
	}
	if sp.setDescription != nil {
		sp.setDescription(sp.Description())
	}
	return nil
}

func (sp *yamlSpecScanner) UnmarshalSpec(u func([]byte) error) (err error) {
	specYaml := cleanupScannerLines(sp.yamlSpec, rxUncommentYAML, nil)
	if len(specYaml) == 0 {
		return errors.New("no spec available to unmarshal")
	}

	if !strings.Contains(specYaml[0], "---") {
		return errors.New("yaml spec has to start with `---`")
	}

	// remove indentation
	specYaml = removeIndent(specYaml)

	// 1. parse yaml lines
	yamlValue := make(map[interface{}]interface{})

	yamlContent := strings.Join(specYaml, "\n")
	err = yaml.Unmarshal([]byte(yamlContent), &yamlValue)
	if err != nil {
		return
	}

	// 2. convert to json
	var jsonValue json.RawMessage
	jsonValue, err = fmts.YAMLToJSON(yamlValue)
	if err != nil {
		return
	}

	// 3. unmarshal the json into an interface
	var data []byte
	data, err = jsonValue.MarshalJSON()
	if err != nil {
		return
	}
	err = u(data)
	if err != nil {
		return
	}

	// all parsed, returning...
	sp.yamlSpec = nil // spec is now consumed, so let's erase the parsed lines
	return
}

// removes indent base on the first line
func removeIndent(spec []string) []string {
	loc := rxIndent.FindStringIndex(spec[0])
	if loc[1] == 0 {
		return spec
	}
	for i := range spec {
		if len(spec[i]) >= loc[1] {
			spec[i] = spec[i][loc[1]-1:]
		}
	}
	return spec
}

// removes indent base on the first line
func removeYamlIndent(spec []string) []string {
	loc := rxIndent.FindStringIndex(spec[0])
	if loc[1] == 0 {
		return nil
	}
	var s []string
	for i := range spec {
		if len(spec[i]) >= loc[1] {
			s = append(s, spec[i][loc[1]-1:])
		}
	}
	return s
}

// aggregates lines in header until it sees a tag.
type sectionedParser struct {
	header     []string
	matched    map[string]tagParser
	annotation valueParser

	seenTag        bool
	skipHeader     bool
	setTitle       func([]string)
	setDescription func([]string)
	workedOutTitle bool
	taggers        []tagParser
	currentTagger  *tagParser
	title          []string
	ignored        bool
}

func (st *sectionedParser) collectTitleDescription() {
	if st.workedOutTitle {
		return
	}
	if st.setTitle == nil {
		st.header = cleanupScannerLines(st.header, rxUncommentHeaders, nil)
		return
	}

	st.workedOutTitle = true
	st.title, st.header = collectScannerTitleDescription(st.header)
}

func (st *sectionedParser) Title() []string {
	st.collectTitleDescription()
	return st.title
}

func (st *sectionedParser) Description() []string {
	st.collectTitleDescription()
	return st.header
}

func (st *sectionedParser) Parse(doc *ast.CommentGroup) error {
	if doc == nil {
		return nil
	}
COMMENTS:
	for _, c := range doc.List {
		for _, line := range strings.Split(c.Text, "\n") {
			if rxSwaggerAnnotation.MatchString(line) {
				if rxIgnoreOverride.MatchString(line) {
					st.ignored = true
					break COMMENTS // an explicit ignore terminates this parser
				}
				if st.annotation == nil || !st.annotation.Matches(line) {
					break COMMENTS // a new swagger: annotation terminates this parser
				}

				_ = st.annotation.Parse([]string{line})
				if len(st.header) > 0 {
					st.seenTag = true
				}
				continue
			}

			var matched bool
			for _, tagger := range st.taggers {
				if tagger.Matches(line) {
					st.seenTag = true
					st.currentTagger = &tagger
					matched = true
					break
				}
			}

			if st.currentTagger == nil {
				if !st.skipHeader && !st.seenTag {
					st.header = append(st.header, line)
				}
				// didn't match a tag, moving on
				continue
			}

			if st.currentTagger.MultiLine && matched {
				// the first line of a multiline tagger doesn't count
				continue
			}

			ts, ok := st.matched[st.currentTagger.Name]
			if !ok {
				ts = *st.currentTagger
			}
			ts.Lines = append(ts.Lines, line)
			if st.matched == nil {
				st.matched = make(map[string]tagParser)
			}
			st.matched[st.currentTagger.Name] = ts

			if !st.currentTagger.MultiLine {
				st.currentTagger = nil
			}
		}
	}
	if st.setTitle != nil {
		st.setTitle(st.Title())
	}
	if st.setDescription != nil {
		st.setDescription(st.Description())
	}
	for _, mt := range st.matched {
		if !mt.SkipCleanUp {
			mt.Lines = cleanupScannerLines(mt.Lines, rxUncommentHeaders, nil)
		}
		if err := mt.Parse(mt.Lines); err != nil {
			return err
		}
	}
	return nil
}

type validationBuilder interface {
	SetMaximum(float64, bool)
	SetMinimum(float64, bool)
	SetMultipleOf(float64)

	SetMinItems(int64)
	SetMaxItems(int64)

	SetMinLength(int64)
	SetMaxLength(int64)
	SetPattern(string)

	SetUnique(bool)
	SetEnum(string)
	SetDefault(interface{})
	SetExample(interface{})
}

type valueParser interface {
	Parse([]string) error
	Matches(string) bool
}

type operationValidationBuilder interface {
	validationBuilder
	SetCollectionFormat(string)
}

type setMaximum struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMaximum) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 2 && len(matches[2]) > 0 {
		max, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return err
		}
		sm.builder.SetMaximum(max, matches[1] == "<")
	}
	return nil
}

func (sm *setMaximum) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type setMinimum struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMinimum) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *setMinimum) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 2 && len(matches[2]) > 0 {
		min, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return err
		}
		sm.builder.SetMinimum(min, matches[1] == ">")
	}
	return nil
}

type setMultipleOf struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMultipleOf) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *setMultipleOf) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 2 && len(matches[1]) > 0 {
		multipleOf, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return err
		}
		sm.builder.SetMultipleOf(multipleOf)
	}
	return nil
}

type setMaxItems struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMaxItems) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *setMaxItems) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		maxItems, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMaxItems(maxItems)
	}
	return nil
}

type setMinItems struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMinItems) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *setMinItems) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		minItems, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMinItems(minItems)
	}
	return nil
}

type setMaxLength struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMaxLength) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		maxLength, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMaxLength(maxLength)
	}
	return nil
}

func (sm *setMaxLength) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type setMinLength struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setMinLength) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		minLength, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMinLength(minLength)
	}
	return nil
}

func (sm *setMinLength) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type setPattern struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sm *setPattern) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		sm.builder.SetPattern(matches[1])
	}
	return nil
}

func (sm *setPattern) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type setCollectionFormat struct {
	builder operationValidationBuilder
	rx      *regexp.Regexp
}

func (sm *setCollectionFormat) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		sm.builder.SetCollectionFormat(matches[1])
	}
	return nil
}

func (sm *setCollectionFormat) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type setUnique struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (su *setUnique) Matches(line string) bool {
	return su.rx.MatchString(line)
}

func (su *setUnique) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := su.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.builder.SetUnique(req)
	}
	return nil
}

type setEnum struct {
	builder validationBuilder
	rx      *regexp.Regexp
}

func (se *setEnum) Matches(line string) bool {
	return se.rx.MatchString(line)
}

func (se *setEnum) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := se.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		se.builder.SetEnum(matches[1])
	}
	return nil
}

func parseValueFromSchema(s string, schema *spec.SimpleSchema) (interface{}, error) {
	if schema != nil {
		switch strings.Trim(schema.TypeName(), "\"") {
		case "integer", "int", "int64", "int32", "int16":
			return strconv.Atoi(s)
		case "bool", "boolean":
			return strconv.ParseBool(s)
		case "number", "float64", "float32":
			return strconv.ParseFloat(s, 64)
		case "object":
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(s), &obj); err != nil {
				// If we can't parse it, just return the string.
				return s, nil
			}
			return obj, nil
		case "array":
			var slice []interface{}
			if err := json.Unmarshal([]byte(s), &slice); err != nil {
				// If we can't parse it, just return the string.
				return s, nil
			}
			return slice, nil
		default:
			return s, nil
		}
	} else {
		return s, nil
	}
}

type setDefault struct {
	scheme  *spec.SimpleSchema
	builder validationBuilder
	rx      *regexp.Regexp
}

func (sd *setDefault) Matches(line string) bool {
	return sd.rx.MatchString(line)
}

func (sd *setDefault) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sd.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		d, err := parseValueFromSchema(matches[1], sd.scheme)
		if err != nil {
			return err
		}
		sd.builder.SetDefault(d)
	}
	return nil
}

type setExample struct {
	scheme  *spec.SimpleSchema
	builder validationBuilder
	rx      *regexp.Regexp
}

func (se *setExample) Matches(line string) bool {
	return se.rx.MatchString(line)
}

func (se *setExample) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := se.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		d, err := parseValueFromSchema(matches[1], se.scheme)
		if err != nil {
			return err
		}
		se.builder.SetExample(d)
	}
	return nil
}

type matchOnlyParam struct {
	tgt *spec.Parameter
	rx  *regexp.Regexp
}

func (mo *matchOnlyParam) Matches(line string) bool {
	return mo.rx.MatchString(line)
}

func (mo *matchOnlyParam) Parse(lines []string) error {
	return nil
}

type setRequiredParam struct {
	tgt *spec.Parameter
}

func (su *setRequiredParam) Matches(line string) bool {
	return rxRequired.MatchString(line)
}

func (su *setRequiredParam) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxRequired.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.tgt.Required = req
	}
	return nil
}

type setReadOnlySchema struct {
	tgt *spec.Schema
}

func (su *setReadOnlySchema) Matches(line string) bool {
	return rxReadOnly.MatchString(line)
}

func (su *setReadOnlySchema) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxReadOnly.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.tgt.ReadOnly = req
	}
	return nil
}

type setDeprecatedOp struct {
	tgt *spec.Operation
}

func (su *setDeprecatedOp) Matches(line string) bool {
	return rxDeprecated.MatchString(line)
}

func (su *setDeprecatedOp) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxDeprecated.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.tgt.Deprecated = req
	}
	return nil
}

type setDiscriminator struct {
	schema *spec.Schema
	field  string
}

func (su *setDiscriminator) Matches(line string) bool {
	return rxDiscriminator.MatchString(line)
}

func (su *setDiscriminator) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxDiscriminator.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		if req {
			su.schema.Discriminator = su.field
		} else if su.schema.Discriminator == su.field {
			su.schema.Discriminator = ""
		}
	}
	return nil
}

type setRequiredSchema struct {
	schema *spec.Schema
	field  string
}

func (su *setRequiredSchema) Matches(line string) bool {
	return rxRequired.MatchString(line)
}

func (su *setRequiredSchema) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxRequired.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		midx := -1
		for i, nm := range su.schema.Required {
			if nm == su.field {
				midx = i
				break
			}
		}
		if req {
			if midx < 0 {
				su.schema.Required = append(su.schema.Required, su.field)
			}
		} else if midx >= 0 {
			su.schema.Required = append(su.schema.Required[:midx], su.schema.Required[midx+1:]...)
		}
	}
	return nil
}

func newMultilineDropEmptyParser(rx *regexp.Regexp, set func([]string)) *multiLineDropEmptyParser {
	return &multiLineDropEmptyParser{
		rx:  rx,
		set: set,
	}
}

type multiLineDropEmptyParser struct {
	set func([]string)
	rx  *regexp.Regexp
}

func (m *multiLineDropEmptyParser) Matches(line string) bool {
	return m.rx.MatchString(line)
}

func (m *multiLineDropEmptyParser) Parse(lines []string) error {
	m.set(removeEmptyLines(lines))
	return nil
}

func newSetSchemes(set func([]string)) *setSchemes {
	return &setSchemes{
		set: set,
		rx:  rxSchemes,
	}
}

type setSchemes struct {
	set func([]string)
	rx  *regexp.Regexp
}

func (ss *setSchemes) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

func (ss *setSchemes) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := ss.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		sch := strings.Split(matches[1], ", ")

		schemes := []string{}
		for _, s := range sch {
			ts := strings.TrimSpace(s)
			if ts != "" {
				schemes = append(schemes, ts)
			}
		}
		ss.set(schemes)
	}
	return nil
}

func newSetSecurity(rx *regexp.Regexp, setter func([]map[string][]string)) *setSecurity {
	return &setSecurity{
		set: setter,
		rx:  rx,
	}
}

type setSecurity struct {
	set func([]map[string][]string)
	rx  *regexp.Regexp
}

func (ss *setSecurity) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

func (ss *setSecurity) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var result []map[string][]string
	for _, line := range lines {
		kv := strings.SplitN(line, ":", 2)
		scopes := []string{}
		var key string

		if len(kv) > 1 {
			scs := strings.Split(kv[1], ",")
			for _, scope := range scs {
				tr := strings.TrimSpace(scope)
				if tr != "" {
					tr = strings.SplitAfter(tr, " ")[0]
					scopes = append(scopes, strings.TrimSpace(tr))
				}
			}

			key = strings.TrimSpace(kv[0])

			result = append(result, map[string][]string{key: scopes})
		}
	}
	ss.set(result)
	return nil
}

func newSetResponses(definitions map[string]spec.Schema, responses map[string]spec.Response, setter func(*spec.Response, map[int]spec.Response)) *setOpResponses {
	return &setOpResponses{
		set:         setter,
		rx:          rxResponses,
		definitions: definitions,
		responses:   responses,
	}
}

type setOpResponses struct {
	set         func(*spec.Response, map[int]spec.Response)
	rx          *regexp.Regexp
	definitions map[string]spec.Schema
	responses   map[string]spec.Response
}

func (ss *setOpResponses) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

//ResponseTag used when specifying a response to point to a defined swagger:response
const ResponseTag = "response"

//BodyTag used when specifying a response to point to a model/schema
const BodyTag = "body"

//DescriptionTag used when specifying a response that gives a description of the response
const DescriptionTag = "description"

func parseTags(line string) (modelOrResponse string, arrays int, isDefinitionRef bool, description string, err error) {
	tags := strings.Split(line, " ")
	parsedModelOrResponse := false

	for i, tagAndValue := range tags {
		tagValList := strings.SplitN(tagAndValue, ":", 2)
		var tag, value string
		if len(tagValList) > 1 {
			tag = tagValList[0]
			value = tagValList[1]
		} else {
			//TODO: Print a warning, and in the long term, do not support not tagged values
			//Add a default tag if none is supplied
			if i == 0 {
				tag = ResponseTag
			} else {
				tag = DescriptionTag
			}
			value = tagValList[0]
		}

		foundModelOrResponse := false
		if !parsedModelOrResponse {
			if tag == BodyTag {
				foundModelOrResponse = true
				isDefinitionRef = true
			}
			if tag == ResponseTag {
				foundModelOrResponse = true
				isDefinitionRef = false
			}
		}
		if foundModelOrResponse {
			//Read the model or response tag
			parsedModelOrResponse = true
			//Check for nested arrays
			arrays = 0
			for strings.HasPrefix(value, "[]") {
				arrays++
				value = value[2:]
			}
			//What's left over is the model name
			modelOrResponse = value
		} else {
			foundDescription := false
			if tag == DescriptionTag {
				foundDescription = true
			}
			if foundDescription {
				//Descriptions are special, they make they read the rest of the line
				descriptionWords := []string{value}
				if i < len(tags)-1 {
					descriptionWords = append(descriptionWords, tags[i+1:]...)
				}
				description = strings.Join(descriptionWords, " ")
				break
			} else {
				if tag == ResponseTag || tag == BodyTag || tag == DescriptionTag {
					err = fmt.Errorf("valid tag %s, but not in a valid position", tag)
				} else {
					err = fmt.Errorf("invalid tag: %s", tag)
				}
				//return error
				return
			}
		}
	}

	//TODO: Maybe do, if !parsedModelOrResponse {return some error}
	return
}

func (ss *setOpResponses) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var def *spec.Response
	var scr map[int]spec.Response

	for _, line := range lines {
		kv := strings.SplitN(line, ":", 2)
		var key, value string

		if len(kv) > 1 {
			key = strings.TrimSpace(kv[0])
			if key == "" {
				// this must be some weird empty line
				continue
			}
			value = strings.TrimSpace(kv[1])
			if value == "" {
				var resp spec.Response
				if strings.EqualFold("default", key) {
					if def == nil {
						def = &resp
					}
				} else {
					if sc, err := strconv.Atoi(key); err == nil {
						if scr == nil {
							scr = make(map[int]spec.Response)
						}
						scr[sc] = resp
					}
				}
				continue
			}
			refTarget, arrays, isDefinitionRef, description, err := parseTags(value)
			if err != nil {
				return err
			}
			//A possible exception for having a definition
			if _, ok := ss.responses[refTarget]; !ok {
				if _, ok := ss.definitions[refTarget]; ok {
					isDefinitionRef = true
				}
			}

			var ref spec.Ref
			if isDefinitionRef {
				if description == "" {
					description = refTarget
				}
				ref, err = spec.NewRef("#/definitions/" + refTarget)
			} else {
				ref, err = spec.NewRef("#/responses/" + refTarget)
			}
			if err != nil {
				return err
			}

			// description should used on anyway.
			resp := spec.Response{ResponseProps: spec.ResponseProps{Description: description}}

			if isDefinitionRef {
				resp.Schema = new(spec.Schema)
				resp.Description = description
				if arrays == 0 {
					resp.Schema.Ref = ref
				} else {
					cs := resp.Schema
					for i := 0; i < arrays; i++ {
						cs.Typed("array", "")
						cs.Items = new(spec.SchemaOrArray)
						cs.Items.Schema = new(spec.Schema)
						cs = cs.Items.Schema
					}
					cs.Ref = ref
				}
				// ref. could be empty while use description tag
			} else if len(refTarget) > 0 {
				resp.Ref = ref
			}

			if strings.EqualFold("default", key) {
				if def == nil {
					def = &resp
				}
			} else {
				if sc, err := strconv.Atoi(key); err == nil {
					if scr == nil {
						scr = make(map[int]spec.Response)
					}
					scr[sc] = resp
				}
			}
		}
	}
	ss.set(def, scr)
	return nil
}

func parseEnum(val string, s *spec.SimpleSchema) []interface{} {
	list := strings.Split(val, ",")
	interfaceSlice := make([]interface{}, len(list))
	for i, d := range list {
		v, err := parseValueFromSchema(d, s)
		if err != nil {
			interfaceSlice[i] = d
			continue
		}

		interfaceSlice[i] = v
	}
	return interfaceSlice
}
