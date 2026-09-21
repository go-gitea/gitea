// Copyright 2026 The Gitea Authors.
// Copyright 2018 https://github.com/citation-file-format/ruby-cff. Licensed under the Apache License, Version 2.0
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

func (d *date) UnmarshalYAML(node *yaml.Node) error {
	d.Time, _ = time.Parse("2006-1-2", node.Value) // unparsable dates are omitted like in ruby-cff
	return nil
}

type licenses []string

func (l *licenses) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*l = licenses{node.Value}
		return nil
	}
	return node.Decode((*[]string)(l))
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
	if node.Kind == yaml.ScalarNode {
		a.isEntity, a.Name = true, node.Value
		return nil
	}
	type plainActor actor
	if err := node.Decode((*plainActor)(a)); err != nil {
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
	Type           string   `yaml:"type"`
	Title          string   `yaml:"title"`
	Authors        []actor  `yaml:"authors"`
	Version        string   `yaml:"version"`
	DOI            string   `yaml:"doi"`
	URL            string   `yaml:"url"`
	RepositoryCode string   `yaml:"repository-code"`
	License        licenses `yaml:"license"`
	DateReleased   date     `yaml:"date-released"`
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
	stringifyScalars(&node)
	var file struct {
		TopLevel          metadata   `yaml:",inline"`
		PreferredCitation *reference `yaml:"preferred-citation"`
	}
	if node.Decode(&file) != nil {
		return "", ""
	}
	ref := file.PreferredCitation
	if ref == nil {
		ref = &reference{metadata: file.TopLevel, isTopLevel: true}
	}
	if ref.Title == "" || len(ref.Authors) == 0 {
		return "", ""
	}
	return formatAPA(ref), formatBibTeX(ref)
}

func stringifyScalars(node *yaml.Node) {
	if node.Kind == yaml.ScalarNode && node.ShortTag() != "!!null" {
		node.Tag = "!!str"
	}
	for _, child := range node.Content {
		stringifyScalars(child)
	}
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

func (r *reference) monthAndYear() (month, year string) {
	if r.Status == "in-press" {
		return "", "in press"
	}
	if r.Year != "" {
		return r.Month, r.Year
	}
	released := r.DateReleased
	if released.IsZero() {
		released = r.DatePublished
	}
	if released.IsZero() {
		return "", ""
	}
	return strconv.Itoa(int(released.Month())), strconv.Itoa(released.Year())
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

func formatAPA(r *reference) string {
	authors := make([]string, 0, len(r.Authors))
	for _, author := range r.Authors {
		authors = append(authors, apaAuthor(author))
	}
	date := apaDate(r)
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
	return joinNonEmpty(". ", combineAuthors(authors), date, r.Title+version+apaTypeLabel(r), apaPublicationData(r), url)
}

func apaAuthor(a actor) string {
	if a.isEntity {
		return a.Name
	}
	name := cmp.Or(a.GivenNames, a.Alias)
	if a.FamilyNames != "" {
		name = a.FamilyNames
		if a.GivenNames != "" {
			name += ", " + initials(a.GivenNames) + "."
		}
	}
	if a.NameParticle != "" {
		name = a.NameParticle + " " + name
	}
	if a.NameSuffix != "" {
		name += ", " + a.NameSuffix
	}
	return name
}

func initials(names string) string {
	parts := strings.Fields(names)
	for i, part := range parts {
		first, _ := utf8.DecodeRuneInString(part)
		parts[i] = string(unicode.ToUpper(first))
	}
	return strings.Join(parts, ". ")
}

func combineAuthors(authors []string) string {
	if len(authors) == 1 {
		return strings.TrimSuffix(authors[0], ".")
	}
	return strings.TrimSuffix(strings.Join(authors[:len(authors)-1], ", ")+", & "+authors[len(authors)-1], ".")
}

func apaDate(r *reference) string {
	if r.Type == "conference-paper" && r.Conference != nil && !r.Conference.DateStart.IsZero() {
		start, end := r.Conference.DateStart, r.Conference.DateEnd
		if end.IsZero() || !start.Before(end.Time) {
			return strconv.Itoa(start.Year())
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
	_, year := r.monthAndYear()
	return year
}

func apaTypeLabel(r *reference) string {
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

func apaPublicationData(r *reference) string {
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
	// letters which don't decompose into ASCII, as transliterated by ruby-cff
	keyLetters = strings.NewReplacer(
		"Æ", "AE", "æ", "ae", "Ð", "D", "ð", "d", "Ø", "O", "ø", "o", "Þ", "Th", "þ", "th", "ß", "ss", "×", "x",
		"Đ", "D", "đ", "d", "Ħ", "H", "ħ", "h", "ı", "i", "Ĳ", "IJ", "ĳ", "ij", "ĸ", "k", "Ŀ", "L", "ŀ", "l",
		"Ł", "L", "ł", "l", "ŉ", "'n", "Ŋ", "NG", "ŋ", "ng", "Œ", "OE", "œ", "oe", "Ŧ", "T", "ŧ", "t",
	)
	keyToASCII     = transform.Chain(norm.NFD, runes.Remove(runes.Predicate(func(r rune) bool { return r > unicode.MaxASCII })))
	keyUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9-]+`)
)

func formatBibTeX(r *reference) string {
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
	if r.Type == "conference-paper" && r.Conference != nil && !r.Conference.DateStart.IsZero() {
		month, year = strconv.Itoa(int(r.Conference.DateStart.Month())), strconv.Itoa(r.Conference.DateStart.Year())
	}
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
		return bibtexEscaper.Replace(strings.Join(r.License, ", "))
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
	for _, a := range actors {
		switch {
		case a.isEntity:
			names = append(names, "{"+bibtexEscaper.Replace(a.Name)+"}")
		case a.FamilyNames == "" && a.GivenNames == "":
			names = append(names, bibtexEscaper.Replace(a.Alias))
		default:
			family := a.FamilyNames
			if a.NameParticle != "" {
				family = a.NameParticle + " " + family
			}
			names = append(names, joinNonEmpty(", ", family, a.NameSuffix, a.GivenNames))
		}
	}
	return strings.Join(names, " and ")
}

func bibtexKey(fields map[string]string) string {
	author, _, _ := strings.Cut(fields["author"], ",")
	titleWords := strings.Fields(fields["title"])
	key := joinNonEmpty("_", author, strings.Join(titleWords[:min(3, len(titleWords))], "_"), fields["year"])
	key, _, _ = transform.String(keyToASCII, keyLetters.Replace(key))
	return strings.Trim(keyUnsafeChars.ReplaceAllString(key, "_"), "_")
}
