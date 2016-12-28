// This is mostly for debugging, to dump all input to a file and keep a trace
// of it, typically to replay it for profiling.

package profile

import (
	"encoding/json"
	"github.com/DataDog/datadog-trace-agent/model"
	"io"
	"sync"
)

// TracesDumper is a generic interface to dump traces for profiling/debugging
type TracesDumper interface {
	// Dump writes an array of traces to a log file
	Dump(traces []model.Trace) error
}

// TracesDump is a simple traces dumper that appends data to a writer
// (typically, a log file)
type TracesDump struct {
	m      sync.Mutex
	writer io.Writer
}

// NewTracesDump creates a TracesDumper which writes data to a standard writer
func NewTracesDump(writer io.Writer) TracesDumper {
	return &TracesDump{writer: writer}
}

// Dump writes an array of traces to a log file
func (td *TracesDump) Dump(traces []model.Trace) error {
	td.m.Lock()
	defer td.m.Unlock()

	enc := json.NewEncoder(td.writer)
	return enc.Encode(traces)
}
