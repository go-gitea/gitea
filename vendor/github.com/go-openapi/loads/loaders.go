package loads

import (
	"encoding/json"
	"errors"
	"net/url"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

var (
	// Default chain of loaders, defined at the package level.
	//
	// By default this matches json and yaml documents.
	//
	// May be altered with AddLoader().
	loaders *loader

	// Default loader. This expects json documents over local or http/https (no matching).
	defaultLoader *loader

	defaultMatcher DocMatcher
)

func init() {
	defaultMatcher = func(_ string) bool { return true }

	defaultLoader = &loader{
		DocLoaderWithMatch: DocLoaderWithMatch{
			Match: defaultMatcher,
			Fn:    JSONDoc,
		},
	}
	loaders = defaultLoader

	AddLoader(swag.YAMLMatcher, swag.YAMLDoc)

	// sets the global default loader for go-openapi/spec
	spec.PathLoader = loaders.Load
}

// DocLoader represents a doc loader type
type DocLoader func(string) (json.RawMessage, error)

// DocMatcher represents a predicate to check if a loader matches
type DocMatcher func(string) bool

// DocLoaderWithMatch describes a loading function for a given extension match.
type DocLoaderWithMatch struct {
	Fn    DocLoader
	Match DocMatcher
}

// NewDocLoaderWithMatch builds a DocLoaderWithMatch to be used in load options
func NewDocLoaderWithMatch(fn DocLoader, matcher DocMatcher) DocLoaderWithMatch {
	return DocLoaderWithMatch{
		Fn:    fn,
		Match: matcher,
	}
}

type loader struct {
	DocLoaderWithMatch
	Next *loader
}

// WithHead adds a loader at the head of the current stack
func (l *loader) WithHead(head *loader) *loader {
	if head == nil {
		return l
	}
	head.Next = l
	return head
}

// WithNext adds a loader at the trail of the current stack
func (l *loader) WithNext(next *loader) *loader {
	l.Next = next
	return next
}

// Load the raw document from path
func (l *loader) Load(path string) (json.RawMessage, error) {
	specURL, erp := url.Parse(path)
	if erp != nil {
		return nil, erp
	}

	var lastErr error = errors.New("no loader matched") // default error if no match was found
	for ldr := l; ldr != nil; ldr = ldr.Next {
		if ldr.Match != nil && !ldr.Match(specURL.Path) {
			continue
		}

		// try then move to next one if there is an error
		b, err := ldr.Fn(path)
		if err == nil {
			return b, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

// JSONDoc loads a json document from either a file or a remote url
func JSONDoc(path string) (json.RawMessage, error) {
	data, err := swag.LoadFromFileOrHTTP(path)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// AddLoader for a document, executed before other previously set loaders.
//
// This sets the configuration at the package level.
//
// The default initial loader at the package level assumes a JSON document.
//
// NOTE:
//  * this updates the default loader used by github.com/go-openapi/spec
//  * since this sets package level globals, you shouln't call this concurrently
//
func AddLoader(predicate DocMatcher, load DocLoader) {
	loaders = loaders.WithHead(&loader{
		DocLoaderWithMatch: DocLoaderWithMatch{
			Match: predicate,
			Fn:    load,
		},
	})

	// sets the global default loader for go-openapi/spec
	spec.PathLoader = loaders.Load
}
