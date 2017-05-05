package config

import (
	"strings"
)

type Option struct {
	// Key preserving original caseness.
	// Use IsKey instead to compare key regardless of caseness.
	Key string
	// Original value as string, could be not notmalized.
	Value string
}

type Options []*Option

// IsKey returns true if the given key matches
// this options' key in a case-insensitive comparison.
func (o *Option) IsKey(key string) bool {
	return strings.ToLower(o.Key) == strings.ToLower(key)
}

// Get gets the value for the given key if set,
// otherwise it returns the empty string.
//
// Note that there is no difference
//
// This matches git behaviour since git v1.8.1-rc1,
// if there are multiple definitions of a key, the
// last one wins.
//
// See: http://article.gmane.org/gmane.linux.kernel/1407184
//
// In order to get all possible values for the same key,
// use GetAll.
func (opts Options) Get(key string) string {
	for i := len(opts) - 1; i >= 0; i-- {
		o := opts[i]
		if o.IsKey(key) {
			return o.Value
		}
	}
	return ""
}

// GetAll returns all possible values for the same key.
func (opts Options) GetAll(key string) []string {
	result := []string{}
	for _, o := range opts {
		if o.IsKey(key) {
			result = append(result, o.Value)
		}
	}
	return result
}

func (opts Options) withoutOption(key string) Options {
	result := Options{}
	for _, o := range opts {
		if !o.IsKey(key) {
			result = append(result, o)
		}
	}
	return result
}

func (opts Options) withAddedOption(key string, value string) Options {
	return append(opts, &Option{key, value})
}

func (opts Options) withSettedOption(key string, value string) Options {
	for i := len(opts) - 1; i >= 0; i-- {
		o := opts[i]
		if o.IsKey(key) {
			result := make(Options, len(opts))
			copy(result, opts)
			result[i] = &Option{key, value}
			return result
		}
	}

	return opts.withAddedOption(key, value)
}
