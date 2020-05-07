// Package tokenizer implements file tokenization used by the enry content
// classifier. This package is an implementation detail of enry and should not
// be imported by other packages.
package tokenizer

// ByteLimit defines the maximum prefix of an input text that will be tokenized.
const ByteLimit = 100000
