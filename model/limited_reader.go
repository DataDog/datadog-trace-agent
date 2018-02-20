package model

import (
	"io"
)

// LimitedReader reads from a reader up to a specific limit. When this limit
// has been reached, any subsequent read will return
// ErrLimitedReaderLimitReached.
// The underlying reader has to implement io.ReadCloser so that it can be used
// with http request bodies.
type LimitedReader struct {
	io.Reader
	io.Closer
}

// NewLimitedReader creates a new LimitedReader.
func NewLimitedReader(r io.ReadCloser, limit int64) *LimitedReader {
	return &LimitedReader{
		Reader: io.LimitReader(r, limit),
		Closer: r,
	}
}
