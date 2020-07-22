package source

const (
	name = "GraphQL"
)

type Source struct {
	Body []byte
	Name string
}

func NewSource(s *Source) *Source {
	if s == nil {
		s = &Source{Name: name}
	}
	if s.Name == "" {
		s.Name = name
	}
	return s
}
