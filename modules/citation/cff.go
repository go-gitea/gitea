// Copyright 2026 The Gitea Authors.
// Copyright 2018-2024 The Ruby Citation File Format Developers. Licensed under the Apache License, Version 2.0
// SPDX-License-Identifier: Apache-2.0

// Package citation formats CITATION.cff files, ported from the formatters of ruby-cff 1.3.0
package citation

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/util"

	"go.yaml.in/yaml/v4"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type date struct{ time.Time }

var dateSeparators = strings.NewReplacer("/", "-", ". ", " ", ".", "-", ",", "")

func (d *date) UnmarshalYAML(node *yaml.Node) error { // the common formats of Ruby's Date.parse, unparsable dates are omitted
	value := dateSeparators.Replace(node.Value)
	for _, layout := range []string{"2006-1-2", "2-1-2006", "2 Jan 2006", "2 January 2006", "Jan 2 2006", "January 2 2006"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			d.Time = parsed
			break
		}
	}
	return nil
}

type license string

func (l *license) UnmarshalYAML(node *yaml.Node) error {
	*l = license(node.Value)
	if node.Kind != yaml.ScalarNode {
		*l = license(rubyInspect(node)) // GitHub outputs a license list with Ruby's Array#to_s
	}
	return nil
}

type actor struct {
	isEntity     bool
	Name         string `yaml:"name"`
	Alias        string `yaml:"alias"`
	FamilyNames  string `yaml:"family-names"`
	GivenNames   string `yaml:"given-names"`
	NameParticle string `yaml:"name-particle"`
	NameSuffix   string `yaml:"name-suffix"`
	Affiliation  string `yaml:"affiliation"`
	City         string `yaml:"city"`
	Region       string `yaml:"region"`
	Country      string `yaml:"country"`
	DateStart    date   `yaml:"date-start"`
	DateEnd      date   `yaml:"date-end"`
}

func (a *actor) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		a.isEntity, a.Name = true, node.Value
		return nil
	case yaml.SequenceNode:
		return nil // ruby-cff can't format an entity given as a list either
	}
	type plainActor actor
	if err := node.Load((*plainActor)(a), yaml.WithUniqueKeys(false)); err != nil {
		return err
	}
	for i := 0; i < len(node.Content); i += 2 {
		a.isEntity = a.isEntity || node.Content[i].Value == "name"
	}
	return nil
}

func (a *actor) name() string {
	if a == nil {
		return ""
	}
	return a.Name
}

type metadata struct {
	Type           string  `yaml:"type"`
	Title          string  `yaml:"title"`
	Authors        []actor `yaml:"authors"`
	Version        string  `yaml:"version"`
	DOI            string  `yaml:"doi"`
	URL            string  `yaml:"url"`
	RepositoryCode string  `yaml:"repository-code"`
	License        license `yaml:"license"`
	DateReleased   date    `yaml:"date-released"`
}

type reference struct {
	metadata        `yaml:",inline"`
	isTopLevel      bool
	Editors         []actor `yaml:"editors"`
	EditorsSeries   []actor `yaml:"editors-series"`
	DatePublished   date    `yaml:"date-published"`
	Year            string  `yaml:"year"`
	Month           string  `yaml:"month"`
	Status          string  `yaml:"status"`
	Journal         string  `yaml:"journal"`
	Volume          string  `yaml:"volume"`
	Issue           string  `yaml:"issue"`
	Start           string  `yaml:"start"`
	End             string  `yaml:"end"`
	ISBN            string  `yaml:"isbn"`
	Notes           string  `yaml:"notes"`
	CollectionTitle string  `yaml:"collection-title"`
	ThesisType      string  `yaml:"thesis-type"`
	Publisher       *actor  `yaml:"publisher"`
	Institution     *actor  `yaml:"institution"`
	Conference      *actor  `yaml:"conference"`
}

// FormatCFF returns the APA and BibTeX citations of a CITATION.cff file, both empty if it has no title or authors
func FormatCFF(content string) (apa, bibtex string) {
	var node yaml.Node
	if yaml.Unmarshal([]byte(content), &node) != nil {
		return "", ""
	}
	normalize(&node)
	var file struct {
		TopLevel          metadata   `yaml:",inline"`
		PreferredCitation *reference `yaml:"preferred-citation"`
	}
	if node.Load(&file, yaml.WithUniqueKeys(false)) != nil { // like Ruby's YAML, the last duplicate key wins
		return "", ""
	}
	ref := file.PreferredCitation
	if ref == nil {
		ref = &reference{metadata: file.TopLevel, isTopLevel: true}
	}
	if ref.Title == "" || len(ref.Authors) == 0 {
		return "", ""
	}
	return ref.formatAPA(), ref.formatBibTeX()
}

var (
	psychTrue  = regexp.MustCompile(`(?i)^(yes|true|on)$`)
	psychFalse = regexp.MustCompile(`(?i)^(no|false|off)$`)
	psychInt   = regexp.MustCompile(`^[-+]?(0b[_,]*[01][01_,]*|0[_,]*[0-7][0-7_,]*|0|[1-9](?:[0-9]|,[0-9]|_[0-9])*|0x[_,]*[0-9a-fA-F][0-9a-fA-F_,]*)$`)
	psychFloat = regexp.MustCompile(`^[-+]?([0-9][0-9_,]*)?\.[0-9]*([eE][-+][0-9]+)?$`)
	psychDigit = strings.NewReplacer(",", "", "_", "")
)

func rubyString(plain string) string { // Ruby's to_s of a plain scalar as Psych resolves it, e.g. version 1.10 becomes 1.1
	switch {
	case psychTrue.MatchString(plain):
		return "true"
	case psychFalse.MatchString(plain):
		return "false"
	case psychFloat.MatchString(plain) && strings.Trim(plain, "+-") != ".":
		if num, err := strconv.ParseFloat(psychDigit.Replace(plain), 64); err == nil {
			formatted := strconv.FormatFloat(num, 'f', -1, 64)
			if !strings.Contains(formatted, ".") {
				formatted += ".0"
			}
			return formatted
		}
	case psychInt.MatchString(plain):
		if num, err := strconv.ParseInt(psychDigit.Replace(plain), 0, 64); err == nil {
			return strconv.FormatInt(num, 10)
		}
	}
	return plain
}

func normalize(node *yaml.Node) { // scalars decode into their Ruby strings, timestamps too, which yaml refuses by default
	if node.Kind == yaml.ScalarNode && node.Style == 0 {
		node.Value = rubyString(node.Value)
	}
	if node.ShortTag() == "!!timestamp" {
		node.Tag = "!!str"
	}
	for _, child := range node.Content {
		normalize(child)
	}
}

func rubyInspect(node *yaml.Node) string {
	var parts []string
	switch node.Kind {
	case yaml.SequenceNode:
		for _, child := range node.Content {
			parts = append(parts, rubyInspect(child))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			parts = append(parts, rubyInspect(node.Content[i])+" => "+rubyInspect(node.Content[i+1]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	if node.ShortTag() == "!!null" {
		return "nil"
	}
	return strconv.Quote(node.Value)
}

var statusNotes = map[string]string{
	"advance-online": "Advance online publication",
	"in-preparation": "Manuscript in preparation.",
	"submitted":      "Manuscript submitted for publication.",
}

func joinNonEmpty(sep string, parts ...string) string {
	return strings.Join(util.SliceRemoveAll(parts, ""), sep)
}

func (r *reference) url() string {
	return cmp.Or(r.RepositoryCode, r.URL)
}

func (r *reference) conferenceDates() (start, end date) {
	if r.Type == "conference-paper" && r.Conference != nil {
		return r.Conference.DateStart, r.Conference.DateEnd
	}
	return date{}, date{}
}

func (r *reference) monthAndYear() (month, year string) {
	when, _ := r.conferenceDates()
	if when.IsZero() {
		if r.Status == "in-press" {
			return "", "in press"
		}
		if r.Year != "" {
			return r.Month, r.Year
		}
		when = cmp.Or(r.DateReleased, r.DatePublished)
	}
	if when.IsZero() {
		return "", ""
	}
	return strconv.Itoa(int(when.Month())), strconv.Itoa(when.Year())
}

func (r *reference) pages(dash string) string {
	if r.Start == "" || r.End == "" || r.Start == r.End {
		return r.Start
	}
	return r.Start + dash + r.End
}

func (r *reference) volume() string {
	if r.Volume == "" || r.Issue == "" {
		return r.Volume
	}
	return r.Volume + "(" + r.Issue + ")"
}

func (r *reference) institution() string {
	if r.Institution != nil {
		return r.Institution.Name
	}
	return r.Authors[0].Affiliation
}

func (r *reference) formatAPA() string {
	authors := make([]string, 0, len(r.Authors))
	for _, author := range r.Authors {
		authors = append(authors, apaAuthor(author))
	}
	date := r.apaDate()
	if date != "" {
		date = "(" + date + ")"
	}
	version := ""
	if r.Version != "" {
		version = " (Version " + r.Version + ")"
	}
	url := r.url()
	if r.DOI != "" {
		url = "https://doi.org/" + r.DOI
	}
	return joinNonEmpty(". ", combineAuthors(authors), date, r.Title+version+r.apaTypeLabel(), r.apaPublicationData(), url)
}

func apaAuthor(author actor) string {
	if author.isEntity {
		return author.Name
	}
	name := cmp.Or(author.GivenNames, author.Alias)
	if author.FamilyNames != "" {
		name = author.FamilyNames
		if author.GivenNames != "" {
			name += ", " + initials(author.GivenNames) + "."
		}
	}
	if author.NameParticle != "" {
		name = author.NameParticle + " " + name
	}
	if author.NameSuffix != "" {
		name += ", " + author.NameSuffix
	}
	return name
}

func initials(names string) string {
	parts := splitWords(names)
	for i, part := range parts {
		first, _ := utf8.DecodeRuneInString(part)
		parts[i] = string(unicode.ToTitle(first))
	}
	return strings.Join(parts, ". ")
}

func splitWords(text string) []string { // like Ruby's String#split, which ignores non-ASCII whitespace
	return strings.FieldsFunc(text, func(char rune) bool { return char <= unicode.MaxASCII && unicode.IsSpace(char) })
}

func combineAuthors(authors []string) string {
	if len(authors) == 1 {
		return strings.TrimSuffix(authors[0], ".")
	}
	return strings.TrimSuffix(strings.Join(authors[:len(authors)-1], ", ")+", & "+authors[len(authors)-1], ".")
}

func (r *reference) apaDate() string {
	start, end := r.conferenceDates()
	if start.IsZero() || end.IsZero() || !start.Before(end.Time) {
		_, year := r.monthAndYear()
		return year
	}
	endLayout := "2"
	if end.Month() != start.Month() {
		endLayout = "January 2"
	}
	if end.Year() != start.Year() {
		endLayout = "2006, " + endLayout
	}
	return start.Format("2006, January 2") + "–" + end.Format(endLayout)
}

func (r *reference) apaTypeLabel() string {
	switch {
	case strings.Contains(r.Type, "data"):
		return " [Data set]"
	case strings.Contains(r.Type, "conference"):
		return " [Conference paper]"
	case !r.isTopLevel && !strings.Contains(r.Type, "software"):
		return ""
	}
	return " [Computer software]"
}

func (r *reference) apaPublicationData() string {
	switch r.Type {
	case "article":
		return joinNonEmpty(", ", r.Journal, r.volume(), r.pages("–"), statusNotes[r.Status])
	case "book":
		return r.Publisher.name()
	case "conference-paper":
		return joinNonEmpty(", ", r.CollectionTitle, r.volume(), r.pages("–"))
	case "report":
		return r.institution()
	case "phdthesis":
		return "[" + cmp.Or(r.ThesisType, "Doctoral dissertation") + ", " + r.institution() + "]"
	case "mastersthesis":
		return "[" + cmp.Or(r.ThesisType, "Master's thesis") + ", " + r.institution() + "]"
	case "unpublished":
		return statusNotes[r.Status]
	}
	return ""
}

var bibtexTypeFields = map[string][]string{
	"article":       {"journal", "note", "number", "pages", "volume"},
	"book":          {"address", "editor", "isbn", "number", "pages", "publisher", "volume"},
	"booklet":       {"address"},
	"inproceedings": {"address", "booktitle", "editor", "pages", "publisher", "series"},
	"manual":        {"address"},
	"mastersthesis": {"address", "school", "type"},
	"misc":          {"pages"},
	"phdthesis":     {"address", "school", "type"},
	"proceedings":   {"address", "booktitle", "editor", "pages", "publisher", "series"},
	"software":      {"license", "version"},
	"techreport":    {"address", "institution", "number"},
	"unpublished":   {"note"},
}

var (
	bibtexEscaper = strings.NewReplacer("&", `\&`, "%", `\%`, "$", `\$`, "#", `\#`, "_", `\_`, "{", `\{`, "}", `\}`)
	keyLetters    = strings.NewReplacer( // letters which don't decompose into ASCII, as transliterated by ruby-cff
		"Æ", "AE", "æ", "ae", "Ð", "D", "ð", "d", "Ø", "O", "ø", "o", "Þ", "Th", "þ", "th", "ß", "ss", "×", "x",
		"Đ", "D", "đ", "d", "Ħ", "H", "ħ", "h", "ı", "i", "Ĳ", "IJ", "ĳ", "ij", "ĸ", "k", "Ŀ", "L", "ŀ", "l",
		"Ł", "L", "ł", "l", "ŉ", "'n", "Ŋ", "NG", "ŋ", "ng", "Œ", "OE", "œ", "oe", "Ŧ", "T", "ŧ", "t",
	)
	keyToASCII = transform.Chain( // ruby-cff only transliterates Latin-1 Supplement, Latin Extended-A and "ệ"
		runes.Remove(runes.Predicate(func(char rune) bool {
			return char > unicode.MaxASCII && (char < 'À' || char > 'ž') && char != 'ệ'
		})),
		norm.NFD,
		runes.Remove(runes.Predicate(func(char rune) bool { return char > unicode.MaxASCII })),
	)
	keyUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9-]+`)
)

func (r *reference) formatBibTeX() string {
	entryType := bibtexType(r.Type)
	fields := map[string]string{
		"author": bibtexActors(r.Authors),
		"title":  "{" + bibtexEscaper.Replace(r.Title) + "}",
		"doi":    bibtexEscaper.Replace(r.DOI),
	}
	for _, name := range bibtexTypeFields[entryType] {
		fields[name] = r.bibtexField(name)
	}
	month, year := r.monthAndYear()
	if num, _ := strconv.Atoi(month); num >= 1 && num <= 12 {
		fields["month"] = strings.ToLower(time.Month(num).String()[:3])
	}
	fields["year"] = year
	fields["url"] = r.url()
	fields["note"] = cmp.Or(fields["note"], r.Notes)
	maps.DeleteFunc(fields, func(_, value string) bool { return value == "" })

	lines := []string{bibtexKey(fields)}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		value := fields[name]
		if name != "month" {
			value = "{" + value + "}"
		}
		lines = append(lines, name+" = "+value)
	}
	return "@" + entryType + "{" + strings.Join(lines, ",\n") + "\n}"
}

func (r *reference) bibtexField(name string) string {
	switch name {
	case "journal":
		return bibtexEscaper.Replace(r.Journal)
	case "volume":
		return bibtexEscaper.Replace(r.Volume)
	case "isbn":
		return bibtexEscaper.Replace(r.ISBN)
	case "license":
		return bibtexEscaper.Replace(string(r.License))
	case "version":
		return bibtexEscaper.Replace(r.Version)
	case "note":
		return statusNotes[r.Status]
	case "number":
		return r.Issue
	case "pages":
		return r.pages("--")
	case "address":
		entity := r.Publisher
		if r.Type == "conference-paper" {
			entity = r.Conference
		}
		if entity == nil {
			return ""
		}
		return joinNonEmpty(", ", entity.City, entity.Region, entity.Country)
	case "editor":
		if len(r.Editors) > 0 {
			return bibtexActors(r.Editors)
		}
		return bibtexActors(r.EditorsSeries)
	case "publisher":
		return bibtexEscaper.Replace(r.Publisher.name())
	case "booktitle":
		return bibtexEscaper.Replace(r.CollectionTitle)
	case "series":
		return bibtexEscaper.Replace(r.Conference.name())
	case "school", "institution":
		return bibtexEscaper.Replace(r.institution())
	case "type":
		return r.ThesisType
	}
	return ""
}

func bibtexType(cffType string) string {
	if cffType == "" || strings.Contains(cffType, "software") {
		return "software"
	}
	switch cffType {
	case "article", "book", "manual", "unpublished", "phdthesis", "mastersthesis":
		return cffType
	case "conference", "proceedings":
		return "proceedings"
	case "conference-paper":
		return "inproceedings"
	case "magazine-article", "newspaper-article":
		return "article"
	case "pamphlet":
		return "booklet"
	case "report":
		return "techreport"
	}
	return "misc"
}

func bibtexActors(actors []actor) string {
	names := make([]string, 0, len(actors))
	for _, entry := range actors {
		switch {
		case entry.isEntity:
			names = append(names, "{"+bibtexEscaper.Replace(entry.Name)+"}")
		case entry.FamilyNames == "" && entry.GivenNames == "":
			names = append(names, bibtexEscaper.Replace(entry.Alias))
		default:
			family := entry.FamilyNames
			if entry.NameParticle != "" {
				family = entry.NameParticle + " " + family
			}
			names = append(names, joinNonEmpty(", ", family, entry.NameSuffix, entry.GivenNames))
		}
	}
	return strings.Join(names, " and ")
}

func bibtexKey(fields map[string]string) string {
	author, _, _ := strings.Cut(fields["author"], ",")
	titleWords := splitWords(fields["title"])
	key := joinNonEmpty("_", author, strings.Join(titleWords[:min(3, len(titleWords))], "_"), fields["year"])
	key, _, _ = transform.String(keyToASCII, keyLetters.Replace(key))
	return strings.Trim(keyUnsafeChars.ReplaceAllString(key, "_"), "_")
}
