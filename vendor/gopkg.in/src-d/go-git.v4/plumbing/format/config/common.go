package config

// New creates a new config instance.
func New() *Config {
	return &Config{}
}

type Config struct {
	Comment  *Comment
	Sections Sections
	Includes Includes
}

type Includes []*Include

// A reference to a included configuration.
type Include struct {
	Path   string
	Config *Config
}

type Comment string

const (
	NoSubsection = ""
)

func (c *Config) Section(name string) *Section {
	for i := len(c.Sections) - 1; i >= 0; i-- {
		s := c.Sections[i]
		if s.IsName(name) {
			return s
		}
	}

	s := &Section{Name: name}
	c.Sections = append(c.Sections, s)
	return s
}

// AddOption is a convenience method to add an option to a given
// section and subsection.
//
// Use the NoSubsection constant for the subsection argument
// if no subsection is wanted.
func (s *Config) AddOption(section string, subsection string, key string, value string) *Config {
	if subsection == "" {
		s.Section(section).AddOption(key, value)
	} else {
		s.Section(section).Subsection(subsection).AddOption(key, value)
	}

	return s
}

// SetOption is a convenience method to set an option to a given
// section and subsection.
//
// Use the NoSubsection constant for the subsection argument
// if no subsection is wanted.
func (s *Config) SetOption(section string, subsection string, key string, value string) *Config {
	if subsection == "" {
		s.Section(section).SetOption(key, value)
	} else {
		s.Section(section).Subsection(subsection).SetOption(key, value)
	}

	return s
}

func (c *Config) RemoveSection(name string) *Config {
	result := Sections{}
	for _, s := range c.Sections {
		if !s.IsName(name) {
			result = append(result, s)
		}
	}

	c.Sections = result
	return c
}

func (c *Config) RemoveSubsection(section string, subsection string) *Config {
	for _, s := range c.Sections {
		if s.IsName(section) {
			result := Subsections{}
			for _, ss := range s.Subsections {
				if !ss.IsName(subsection) {
					result = append(result, ss)
				}
			}
			s.Subsections = result
		}
	}

	return c
}
