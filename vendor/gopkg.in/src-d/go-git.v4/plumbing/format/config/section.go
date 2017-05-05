package config

import "strings"

type Section struct {
	Name        string
	Options     Options
	Subsections Subsections
}

type Subsection struct {
	Name    string
	Options Options
}

type Sections []*Section

type Subsections []*Subsection

func (s *Section) IsName(name string) bool {
	return strings.ToLower(s.Name) == strings.ToLower(name)
}

func (s *Section) Option(key string) string {
	return s.Options.Get(key)
}

func (s *Section) AddOption(key string, value string) *Section {
	s.Options = s.Options.withAddedOption(key, value)
	return s
}

func (s *Section) SetOption(key string, value string) *Section {
	s.Options = s.Options.withSettedOption(key, value)
	return s
}

func (s *Section) RemoveOption(key string) *Section {
	s.Options = s.Options.withoutOption(key)
	return s
}

func (s *Section) Subsection(name string) *Subsection {
	for i := len(s.Subsections) - 1; i >= 0; i-- {
		ss := s.Subsections[i]
		if ss.IsName(name) {
			return ss
		}
	}

	ss := &Subsection{Name: name}
	s.Subsections = append(s.Subsections, ss)
	return ss
}

func (s *Section) HasSubsection(name string) bool {
	for _, ss := range s.Subsections {
		if ss.IsName(name) {
			return true
		}
	}

	return false
}

func (s *Subsection) IsName(name string) bool {
	return s.Name == name
}

func (s *Subsection) Option(key string) string {
	return s.Options.Get(key)
}

func (s *Subsection) AddOption(key string, value string) *Subsection {
	s.Options = s.Options.withAddedOption(key, value)
	return s
}

func (s *Subsection) SetOption(key string, value string) *Subsection {
	s.Options = s.Options.withSettedOption(key, value)
	return s
}

func (s *Subsection) RemoveOption(key string) *Subsection {
	s.Options = s.Options.withoutOption(key)
	return s
}
