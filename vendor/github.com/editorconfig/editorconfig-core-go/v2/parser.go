package editorconfig

// Parser interface is responsible for the parsing of the ini file and the
// globbing patterns.
type Parser interface {
	// ParseIni takes one .editorconfig (ini format) filename and returns its
	// Editorconfig definition.
	ParseIni(filename string) (*Editorconfig, error)

	// FnmatchCase takes a pattern, a filename, and tells wether the given filename
	// matches the globbing pattern.
	FnmatchCase(pattern string, filename string) (bool, error)
}
