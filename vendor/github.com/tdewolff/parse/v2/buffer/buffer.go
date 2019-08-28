/*
Package buffer contains buffer and wrapper types for byte slices. It is useful for writing lexers or other high-performance byte slice handling.

The `Reader` and `Writer` types implement the `io.Reader` and `io.Writer` respectively and provide a thinner and faster interface than `bytes.Buffer`.
The `Lexer` type is useful for building lexers because it keeps track of the start and end position of a byte selection, and shifts the bytes whenever a valid token is found.
The `StreamLexer` does the same, but keeps a buffer pool so that it reads a limited amount at a time, allowing to parse from streaming sources.
*/
package buffer

// defaultBufSize specifies the default initial length of internal buffers.
var defaultBufSize = 4096

// MinBuf specifies the default initial length of internal buffers.
// Solely here to support old versions of parse.
var MinBuf = defaultBufSize
