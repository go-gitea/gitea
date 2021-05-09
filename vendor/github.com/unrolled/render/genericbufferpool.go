package render

import "bytes"

// GenericBufferPool abstracts buffer pool implementations
type GenericBufferPool interface {
	Get() *bytes.Buffer
	Put(*bytes.Buffer)
}
